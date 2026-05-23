package jooble

import (
	"context"
	"fmt"
	"hash/fnv"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/util"
)

const (
	searchURL    = "https://jooble.org/Search"
	pageSize     = 20
	maxPages     = 100
	resultsLimit = 15
)

// Browser-like headers to reduce bot detection.
var defaultHeaders = map[string]string{
	"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7",
	"Accept-Language": "en-US,en;q=0.9",
	"Accept-Encoding": "gzip, deflate, br",
	"User-Agent":      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36",
	"Cache-Control":   "no-cache",
}

// Field-level regex patterns matching Jooble's HTML structure.
// Jooble job listings use structured card elements with class names
// that are stable across pages.
var (
	jobCardRe   = regexp.MustCompile(`<a[^>]*class="[^"]*job-title[^"]*"[^>]*href="([^"]+)"[^>]*>\s*([^<]+)\s*</a>`)
	companyRe   = regexp.MustCompile(`<span[^>]*class="[^"]*company[^"]*"[^>]*>\s*([^<]+)\s*</span>`)
	locationRe  = regexp.MustCompile(`<span[^>]*class="[^"]*location[^"]*"[^>]*>\s*([^<]+)\s*</span>`)
	salaryRe    = regexp.MustCompile(`<span[^>]*class="[^"]*salary[^"]*"[^>]*>\s*([^<]+)\s*</span>`)
	descRe      = regexp.MustCompile(`<div[^>]*class="[^"]*(?:description|snippet)[^"]*"[^>]*>\s*([^<]+)\s*</div>`)
	dateRe      = regexp.MustCompile(`<span[^>]*class="[^"]*date[^"]*"[^>]*>\s*([^<]+)\s*</span>`)
	typeRe      = regexp.MustCompile(`<span[^>]*class="[^"]*job-type[^"]*"[^>]*>\s*([^<]+)\s*</span>`)
	remoteRe    = regexp.MustCompile(`remote|work\s*from\s*home|wfh`)
	jobCardCount = regexp.MustCompile(`class="[^"]*job-card[^"]*"`)
)

// Scraper implements the scraper.Scraper interface for Jooble.
type Scraper struct {
	client    *http.Client
	searchURL string
}

// New creates a new Jooble scraper. If client is nil, a default
// HTTP client with retries and timeout is created.
func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Retries: 2, Timeout: 30 * time.Second})
	}
	return &Scraper{client: client, searchURL: searchURL}
}

// NewWithURLs creates a scraper with a custom base URL override for testing.
func NewWithURLs(client *http.Client, endpoint string) *Scraper {
	s := New(client)
	if strings.TrimSpace(endpoint) != "" {
		s.searchURL = strings.TrimSpace(endpoint)
	}
	return s
}

// SiteName returns model.SiteJooble.
func (s *Scraper) SiteName() model.Site { return model.SiteJooble }

// Scrape fetches job listings from Jooble's HTML search pages.
// It handles page-based pagination via the page parameter, respects
// context cancellation, and rate-limits to ~3 requests/second.
func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	wanted := input.ResultsWanted
	if wanted <= 0 {
		wanted = resultsLimit
	}

	searchTerm := strings.TrimSpace(input.SearchTerm)
	if searchTerm == "" {
		return nil, fmt.Errorf("jooble: search term required")
	}

	jobs := make([]model.JobPost, 0, wanted)
	page := 1
	maxPagesToFetch := maxPages
	if wanted > 0 {
		maxPagesToFetch = min(maxPages, (wanted+pageSize-1)/pageSize+1)
	}

	// Rate limiter: max 3 requests per second (~333ms minimum interval).
	rateLimiter := time.NewTicker(334 * time.Millisecond)
	defer rateLimiter.Stop()

	seen := make(map[string]struct{})

	for len(jobs) < wanted && page <= maxPagesToFetch {
		select {
		case <-ctx.Done():
			return jobs, ctx.Err()
		default:
		}

		body, err := s.fetchPage(ctx, searchTerm, input.Location, page)
		if err != nil {
			return nil, fmt.Errorf("jooble page %d: %w", page, err)
		}

		pageJobs := parseJobs(body)
		if len(pageJobs) == 0 {
			break
		}

		for _, j := range pageJobs {
			if j.ID == "" || j.Title == "" {
				continue
			}
			if _, exists := seen[j.ID]; exists {
				continue
			}
			seen[j.ID] = struct{}{}
			jobs = append(jobs, j)
			if len(jobs) >= wanted {
				break
			}
		}

		page++

		// Rate-limit before next request.
		select {
		case <-ctx.Done():
			return jobs, ctx.Err()
		case <-rateLimiter.C:
		}
	}

	if !util.HasMeaningfulJobs(jobs) {
		return nil, fmt.Errorf("jooble no parseable jobs")
	}
	if len(jobs) > wanted {
		jobs = jobs[:wanted]
	}
	return jobs, nil
}

// fetchPage makes one HTTP request to a Jooble search page.
func (s *Scraper) fetchPage(ctx context.Context, searchTerm, location string, page int) ([]byte, error) {
	u, err := url.Parse(s.searchURL)
	if err != nil {
		return nil, fmt.Errorf("jooble url: %w", err)
	}

	q := u.Query()
	q.Set("keyword", searchTerm)
	if strings.TrimSpace(location) != "" {
		q.Set("location", strings.TrimSpace(location))
	}
	if page > 1 {
		q.Set("page", fmt.Sprintf("%d", page))
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	setDefaultHeaders(req)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("jooble request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("jooble status %d", resp.StatusCode)
	}

	body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("jooble read: %w", err)
	}
	return body, nil
}

// setDefaultHeaders applies browser-like headers to a request.
func setDefaultHeaders(req *http.Request) {
	for k, v := range defaultHeaders {
		if req.Header.Get(k) == "" {
			req.Header.Set(k, v)
		}
	}
}

// parseJobs extracts job postings from Jooble HTML.
// It uses ordered field extraction: finds all titles/URLs, companies, locations,
// salaries, descriptions, dates, and types in the order they appear, then zips
// them into JobPost records.
func parseJobs(html []byte) []model.JobPost {
	raw := string(html)

	// Extract job card count from class markers.
	cardCount := len(jobCardCount.FindAllString(raw, -1))
	if cardCount == 0 {
		cardCount = 20 // fallback: scan all matches and zip
	}
	if cardCount > 25 {
		cardCount = 25
	}

	jobCards := findJobCards(jobCardRe, raw)
	companies := findAll(companyRe, raw)
	locations := findAll(locationRe, raw)
	salaries := findAll(salaryRe, raw)
	descriptions := findAll(descRe, raw)
	dates := findAll(dateRe, raw)
	types := findAll(typeRe, raw)

	// Use the number of captured job card pairs (title + URL) as our actual count.
	if n := len(jobCards); n > 0 && n < cardCount {
		cardCount = n
	}

	jobs := make([]model.JobPost, 0, cardCount)
	for i := 0; i < cardCount; i++ {
		var title, jobURL string
		if i < len(jobCards) {
			title = jobCards[i].title
			jobURL = jobCards[i].url
		}
		if title == "" {
			continue
		}

		company := ""
		if i < len(companies) {
			company = companies[i]
		}

		loc := model.Location{}
		if i < len(locations) && locations[i] != "" {
			loc = parseLocation(locations[i])
		}

		salary := ""
		if i < len(salaries) {
			salary = salaries[i]
		}

		desc := ""
		if i < len(descriptions) {
			desc = descriptions[i]
		}

		id := "jb-" + hashID(jobURL + title + company)

		isRemote := remoteRe.MatchString(raw) ||
			strings.Contains(strings.ToLower(loc.City), "remote") ||
			strings.Contains(strings.ToLower(loc.State), "remote")

		jobType := ""
		if i < len(types) {
			jobType = normalizeJobType(types[i])
		}

		datePosted := parseDate(dates, i)

		comp := parseSalary(salary)

		job := model.JobPost{
			ID:           id,
			Title:        title,
			CompanyName:  company,
			Location:     loc,
			JobURL:       jobURL,
			Description:  desc,
			DatePosted:   datePosted,
			IsRemote:     isRemote,
			JobType:      jobType,
			Compensation: comp,
		}
		jobs = append(jobs, job)
	}

	return jobs
}

type jobCard struct {
	title string
	url   string
}

// findJobCards extracts job title + URL pairs from anchor elements
// identified as job title links.
func findJobCards(re *regexp.Regexp, html string) []jobCard {
	matches := re.FindAllStringSubmatch(html, -1)
	out := make([]jobCard, 0, len(matches))
	for _, m := range matches {
		if len(m) < 3 {
			continue
		}
		u := strings.TrimSpace(m[1])
		t := strings.TrimSpace(m[2])
		if u == "" || t == "" || strings.Contains(strings.ToLower(t), "javascript") {
			continue
		}
		// Make relative URLs absolute.
		if strings.HasPrefix(u, "/") {
			u = "https://jooble.org" + u
		}
		out = append(out, jobCard{title: t, url: u})
	}
	return out
}

// findAll extracts all submatch-1 values from the regex applied to html,
// in document order.
func findAll(re *regexp.Regexp, html string) []string {
	matches := re.FindAllStringSubmatch(html, -1)
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		if len(m) >= 2 {
			v := strings.TrimSpace(m[1])
			if v != "" {
				out = append(out, v)
			}
		}
	}
	return out
}

// parseLocation splits a "City, State" or "City" location string.
func parseLocation(v string) model.Location {
	parts := strings.SplitN(v, ", ", 2)
	loc := model.Location{}
	if len(parts) > 0 {
		loc.City = strings.TrimSpace(parts[0])
	}
	if len(parts) > 1 {
		loc.State = strings.TrimSpace(parts[1])
	}
	return loc
}

// parseDate extracts a date from Jooble's posted-date format.
// Common formats: "Posted 2 days ago", "Today", "3 days ago", ISO date.
func parseDate(dates []string, i int) *time.Time {
	if i >= len(dates) || dates[i] == "" {
		return nil
	}
	text := strings.ToLower(strings.TrimSpace(dates[i]))
	text = strings.TrimPrefix(text, "posted")
	text = strings.TrimSpace(text)

	now := time.Now()

	switch {
	case strings.Contains(text, "today"):
		t := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		return &t
	case strings.Contains(text, "yesterday"):
		t := time.Date(now.Year(), now.Month(), now.Day()-1, 0, 0, 0, 0, now.Location())
		return &t
	case strings.Contains(text, "day") || strings.Contains(text, "days"):
		var days int
		if _, err := fmt.Sscanf(text, "%d", &days); err == nil && days > 0 {
			t := time.Date(now.Year(), now.Month(), now.Day()-days, 0, 0, 0, 0, now.Location())
			return &t
		}
	case strings.Contains(text, "week") || strings.Contains(text, "weeks"):
		var weeks int
		if _, err := fmt.Sscanf(text, "%d", &weeks); err == nil && weeks > 0 {
			t := now.AddDate(0, 0, -weeks*7)
			return &t
		}
	case strings.Contains(text, "month") || strings.Contains(text, "months"):
		var months int
		if _, err := fmt.Sscanf(text, "%d", &months); err == nil && months > 0 {
			t := now.AddDate(0, -months, 0)
			return &t
		}
	default:
		// Try parsing as an ISO date
		if t, err := time.Parse("2006-01-02", text); err == nil {
			return &t
		}
	}
	return nil
}

// ─── Salary Parsing ────────────────────────────────────────────────────────
// Ported from the TypeScript Jooble source. Parses salary strings like
// "$80,000 - $120,000", "€3,000 - €5,000 monthly", "$50/hr", etc.

var salaryRangeRe = regexp.MustCompile(`[$€£₹]?\s*(\d{1,3}(?:,\d{3})*|\d+)\s*[–-]\s*[$€£₹]?\s*(\d{1,3}(?:,\d{3})*|\d+)`)

var currencySymbolMap = map[string]string{
	"$": "USD",
	"€": "EUR",
	"£": "GBP",
	"₹": "INR",
}

type intervalMatch struct {
	pattern  *regexp.Regexp
	interval model.CompensationInterval
}

var intervalKeywords = []intervalMatch{
	{regexp.MustCompile(`(?i)(?:per\s+hour|/\s*hr|hourly|/\s*hour)`), model.IntervalHourly},
	{regexp.MustCompile(`(?i)(?:per\s+day|/\s*day|daily)`), model.IntervalDaily},
	{regexp.MustCompile(`(?i)(?:per\s+week|/\s*wk|weekly)`), model.IntervalWeekly},
	{regexp.MustCompile(`(?i)(?:per\s+month|/\s*mo|monthly|p\.?\s*m\.?)`), model.IntervalMonthly},
	{regexp.MustCompile(`(?i)(?:per\s+year|/\s*yr|yearly|annual|annually|p\.?\s*a\.?)`), model.IntervalYearly},
}

// parseSalary parses a salary string into a Compensation struct.
// Detects currency from symbols and interval from keywords.
// Falls back to nil for unparseable strings.
func parseSalary(salary string) *model.Compensation {
	salary = strings.TrimSpace(salary)
	if salary == "" {
		return nil
	}

	currency := detectCurrency(salary)
	interval := detectInterval(salary)

	// Try range first: "$80,000 - $120,000"
	matches := salaryRangeRe.FindStringSubmatch(salary)
	if len(matches) >= 3 {
		minAmt := parseFloatAmount(matches[1])
		maxAmt := parseFloatAmount(matches[2])
		if minAmt > 0 && maxAmt > 0 {
			return &model.Compensation{
				Interval:  interval,
				MinAmount: &minAmt,
				MaxAmount: &maxAmt,
				Currency:  currency,
			}
		}
	}

	// Try single number: "$50,000"
	singleRe := regexp.MustCompile(`[$€£₹]?\s*(\d{1,3}(?:,\d{3})*|\d+)`)
	m := singleRe.FindStringSubmatch(salary)
	if len(m) >= 2 {
		amt := parseFloatAmount(m[1])
		if amt > 0 {
			return &model.Compensation{
				Interval:  interval,
				MinAmount: &amt,
				Currency:  currency,
			}
		}
	}

	return nil
}

// detectCurrency returns the ISO 4217 code detected from currency symbols in the string.
// Returns "USD" as default when no symbol is found.
func detectCurrency(salary string) string {
	for sym, code := range currencySymbolMap {
		if strings.Contains(salary, sym) {
			return code
		}
	}
	return "USD"
}

// detectInterval returns the compensation interval detected from keywords.
// Returns YEARLY as default.
func detectInterval(salary string) model.CompensationInterval {
	for _, im := range intervalKeywords {
		if im.pattern.MatchString(salary) {
			return im.interval
		}
	}
	return model.IntervalYearly
}

// parseFloatAmount converts a string like "80,000" to float64 80000.
func parseFloatAmount(s string) float64 {
	s = strings.ReplaceAll(s, ",", "")
	s = strings.TrimSpace(s)
	var v float64
	if _, err := fmt.Sscanf(s, "%f", &v); err == nil {
		return v
	}
	return 0
}

// normalizeJobType maps Jooble's job type strings to standard values.
func normalizeJobType(t string) string {
	lower := strings.ToLower(strings.TrimSpace(t))
	switch {
	case strings.Contains(lower, "full"), strings.Contains(lower, "permanent"):
		return "fulltime"
	case strings.Contains(lower, "part"):
		return "parttime"
	case strings.Contains(lower, "contract"), strings.Contains(lower, "temporary"):
		return "contract"
	case strings.Contains(lower, "intern"):
		return "internship"
	default:
		return lower
	}
}

// hashID generates a stable hash string for deduplication.
func hashID(s string) string {
	h := fnv.New64a()
	h.Write([]byte(s))
	return fmt.Sprintf("%d", h.Sum64())
}
