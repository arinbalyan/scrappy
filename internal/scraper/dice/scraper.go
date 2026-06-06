package dice

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/util"
)

const (
	apiURL    = "https://job-search-api.svc.dhigroupinc.com/v1/dice/jobs/search"
	searchURL = "https://www.dice.com/jobs"
	pageSize  = 20
	maxPages  = 50
)

// Default API headers. The x-api-key is a publicly known client-side key
// used by dice.com (same as in the TypeScript source).
var apiHeaders = map[string]string{
	"Accept":       "application/json",
	"Content-Type": "application/json",
	"x-api-key":    "1YAt0R9wBg4WfsF9VB2778F5CHLAPMVW3WAZcKd8",
	"User-Agent":   "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
}

// Browser-like headers for HTML fallback scraping.
var htmlHeaders = map[string]string{
	"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8",
	"Accept-Language": "en-US,en;q=0.9",
	"User-Agent":      "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
}

// Regex patterns for HTML fallback parsing (matches Dice's data-cy attributes).
var (
	titleRe    = regexp.MustCompile(`data-cy="card-title"[^>]*>\s*([^<]+)\s*<`)
	companyRe  = regexp.MustCompile(`data-cy="search-result-company-name"[^>]*>\s*([^<]+)\s*<`)
	locationRe = regexp.MustCompile(`data-cy="search-result-location"[^>]*>\s*([^<]+)\s*<`)
	salaryRe   = regexp.MustCompile(`data-cy="search-result-salary"[^>]*>\s*([^<]+)\s*<`)
	dateRe     = regexp.MustCompile(`data-cy="card-posted-date"[^>]*>\s*([^<]+)\s*<`)
	jobURLRe   = regexp.MustCompile(`href="(/job-detail/[^"]+)"`)
	titleHrefRe = regexp.MustCompile(`data-cy="card-title"[^>]*href="([^"]+)"`)
	cardRe     = regexp.MustCompile(`data-cy="search-card"`)

	salaryRangeRe = regexp.MustCompile(`\$?([0-9,]+(?:\.\d+)?)\s*[-–to]+\s*\$?([0-9,]+(?:\.\d+)?)`)

	reNextData = regexp.MustCompile(`(?is)<script[^>]*id=["']__NEXT_DATA__["'][^>]*>(.*?)</script>`)
)

// ---------- Dice REST API JSON types (mirrors dice.types.ts) ----------

type diceAPIResponse struct {
	Data    []diceAPIJob `json:"data"`
	Meta    *diceMeta    `json:"meta,omitempty"`
	QueryID string       `json:"queryId,omitempty"`
}

type diceMeta struct {
	TotalHits int `json:"totalHits,omitempty"`
	Page      int `json:"page,omitempty"`
	PageSize  int `json:"pageSize,omitempty"`
}

type diceAPIJob struct {
	ID                string        `json:"id"`
	JobID             string        `json:"jobId,omitempty"`
	Title             string        `json:"title"`
	CompanyName       string        `json:"companyName,omitempty"`
	Summary           string        `json:"summary,omitempty"`
	DetailsPageURL    string        `json:"detailsPageUrl,omitempty"`
	FormattedLocation string        `json:"formattedLocation,omitempty"`
	PostedDate        string        `json:"postedDate,omitempty"`
	ModifiedDate      string        `json:"modifiedDate,omitempty"`
	JobLocation       *diceLocation `json:"jobLocation,omitempty"`
	Salary            string        `json:"salary,omitempty"`
	PayRateRange      *dicePayRange `json:"payRateRange,omitempty"`
	EmploymentType    string        `json:"employmentType,omitempty"`
	IsRemote          bool          `json:"isRemote,omitempty"`
}

type diceLocation struct {
	DisplayName string  `json:"displayName,omitempty"`
	Latitude    float64 `json:"latitude,omitempty"`
	Longitude   float64 `json:"longitude,omitempty"`
}

type dicePayRange struct {
	Min *float64 `json:"min,omitempty"`
	Max *float64 `json:"max,omitempty"`
}

// Scraper implements the scraper.Scraper interface for Dice (dice.com).
// Primary approach: REST API. Fallback: HTML scraping with regex.
// Rate limit: 3 requests per second.
type Scraper struct {
	client     *http.Client
	apiBaseURL string
	searchURL  string
}

// New creates a new Dice scraper. If client is nil, a default HTTP client
// with retries and timeout is created.
func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Retries: 2, Timeout: 30 * time.Second})
	}
	return &Scraper{
		client:     client,
		apiBaseURL: apiURL,
		searchURL:  searchURL,
	}
}

// NewWithURLs creates a scraper with custom endpoint URLs for testing.
func NewWithURLs(client *http.Client, apiEndpoint, htmlEndpoint string) *Scraper {
	s := New(client)
	if strings.TrimSpace(apiEndpoint) != "" {
		s.apiBaseURL = strings.TrimSpace(apiEndpoint)
	}
	if strings.TrimSpace(htmlEndpoint) != "" {
		s.searchURL = strings.TrimSpace(htmlEndpoint)
	}
	return s
}

// SiteName returns model.SiteDice.
func (s *Scraper) SiteName() model.Site { return model.SiteDice }

// Scrape fetches job listings from Dice.
// Primary approach: REST API. If the API returns zero jobs or fails,
// falls back to HTML scraping. Rate limited to 3 requests/second.
func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	wanted := input.ResultsWanted
	if wanted <= 0 {
		wanted = 15
	}

	searchTerm := strings.TrimSpace(input.SearchTerm)
	if searchTerm == "" {
		return nil, fmt.Errorf("dice: search term required")
	}

	// Primary: REST API
	apiJobs, apiErr := s.scrapeWithAPI(ctx, input, searchTerm, wanted)
	if apiErr == nil && len(apiJobs) > 0 {
		// Fetch full descriptions from detail pages to enable email extraction.
		s.enrichWithFullDescriptions(ctx, apiJobs)
		if !util.HasMeaningfulJobs(apiJobs) {
			return nil, fmt.Errorf("dice no parseable jobs")
		}
		if len(apiJobs) > wanted {
			apiJobs = apiJobs[:wanted]
		}
		return apiJobs, nil
	}

	// Fallback: HTML scraping
	htmlJobs, htmlErr := s.scrapeWithHTML(ctx, input, searchTerm, wanted)
	if htmlErr == nil && len(htmlJobs) > 0 {
		if !util.HasMeaningfulJobs(htmlJobs) {
			return nil, fmt.Errorf("dice no parseable jobs from html fallback")
		}
		if len(htmlJobs) > wanted {
			htmlJobs = htmlJobs[:wanted]
		}
		return htmlJobs, nil
	}

	// If both failed, return the api error if any, otherwise a generic one
	if apiErr != nil {
		return nil, fmt.Errorf("dice: %w", apiErr)
	}
	return nil, fmt.Errorf("dice no jobs found")
}

// scrapeWithAPI fetches jobs using the Dice REST API.
func (s *Scraper) scrapeWithAPI(ctx context.Context, input model.ScraperInput, searchTerm string, wanted int) ([]model.JobPost, error) {
	jobs := make([]model.JobPost, 0, wanted)
	page := 1
	maxPagesToFetch := min(maxPages, (wanted+pageSize-1)/pageSize+1)

	// Rate limiter: 3 requests per second (~333ms interval).
	rateLimiter := time.NewTicker(333 * time.Millisecond)
	defer rateLimiter.Stop()

	for len(jobs) < wanted && page <= maxPagesToFetch {
		select {
		case <-ctx.Done():
			return jobs, ctx.Err()
		default:
		}

		body, err := s.fetchAPIPage(ctx, input, searchTerm, page)
		if err != nil {
			return nil, fmt.Errorf("dice api page %d: %w", page, err)
		}

		var parsed diceAPIResponse
		if err := json.Unmarshal(body, &parsed); err != nil {
			return nil, fmt.Errorf("dice api decode: %w", err)
		}

		if len(parsed.Data) == 0 {
			break
		}

		for _, r := range parsed.Data {
			if len(jobs) >= wanted {
				break
			}
			job := convertAPIToJob(r)
			if job.Title == "" {
				continue
			}
			jobs = append(jobs, job)
		}

		// If fewer results than page size, no more pages.
		if len(parsed.Data) < pageSize {
			break
		}

		page++

		// Rate-limit before next request.
		select {
		case <-ctx.Done():
			return jobs, ctx.Err()
		case <-rateLimiter.C:
		}
	}

	return jobs, nil
}

// fetchAPIPage makes one HTTP request to the Dice REST API.
func (s *Scraper) fetchAPIPage(ctx context.Context, input model.ScraperInput, searchTerm string, page int) ([]byte, error) {
	u, err := url.Parse(s.apiBaseURL)
	if err != nil {
		return nil, fmt.Errorf("dice url: %w", err)
	}

	q := u.Query()
	q.Set("q", searchTerm)
	if v := strings.TrimSpace(input.Location); v != "" {
		q.Set("location", v)
	}
	q.Set("page", strconv.Itoa(page))
	q.Set("pageSize", strconv.Itoa(pageSize))
	q.Set("countryCode2", "US")
	q.Set("language", "en")
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	setAPIHeaders(req)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("dice api request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("dice api status %d", resp.StatusCode)
	}

	body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("dice api read: %w", err)
	}
	return body, nil
}

// scrapeWithHTML fetches jobs by scraping Dice's HTML search pages.
func (s *Scraper) scrapeWithHTML(ctx context.Context, input model.ScraperInput, searchTerm string, wanted int) ([]model.JobPost, error) {
	jobs := make([]model.JobPost, 0, wanted)
	page := 1
	maxPagesToFetch := min(maxPages, (wanted+pageSize-1)/pageSize+1)

	// Rate limiter: 3 requests per second (~333ms interval).
	rateLimiter := time.NewTicker(333 * time.Millisecond)
	defer rateLimiter.Stop()

	for len(jobs) < wanted && page <= maxPagesToFetch {
		select {
		case <-ctx.Done():
			return jobs, ctx.Err()
		default:
		}

		body, err := s.fetchHTMLPage(ctx, input, searchTerm, page)
		if err != nil {
			return nil, fmt.Errorf("dice html page %d: %w", page, err)
		}

		pageJobs := parseHTMLJobs(body)
		if len(pageJobs) == 0 {
			break
		}

		for _, j := range pageJobs {
			if j.Title == "" || j.ID == "" {
				continue
			}
			jobs = append(jobs, j)
			if len(jobs) >= wanted {
				break
			}
		}

		if len(pageJobs) < pageSize {
			break
		}

		page++

		select {
		case <-ctx.Done():
			return jobs, ctx.Err()
		case <-rateLimiter.C:
		}
	}

	return jobs, nil
}

// fetchHTMLPage makes one HTTP request to a Dice HTML search page.
func (s *Scraper) fetchHTMLPage(ctx context.Context, input model.ScraperInput, searchTerm string, page int) ([]byte, error) {
	u, err := url.Parse(s.searchURL)
	if err != nil {
		return nil, fmt.Errorf("dice url: %w", err)
	}

	q := u.Query()
	q.Set("q", searchTerm)
	if v := strings.TrimSpace(input.Location); v != "" {
		q.Set("location", v)
	}
	q.Set("page", strconv.Itoa(page))
	q.Set("pageSize", strconv.Itoa(pageSize))
	q.Set("countryCode", "US")
	q.Set("language", "en")
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	setHTMLHeaders(req)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("dice html request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("dice html status %d", resp.StatusCode)
	}

	body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("dice html read: %w", err)
	}
	return body, nil
}

// ---------- Helpers ----------

// setAPIHeaders applies Dice REST API headers to a request.
func setAPIHeaders(req *http.Request) {
	for k, v := range apiHeaders {
		req.Header.Set(k, v)
	}
}

// setHTMLHeaders applies browser-like headers for HTML scraping.
func setHTMLHeaders(req *http.Request) {
	for k, v := range htmlHeaders {
		req.Header.Set(k, v)
	}
}

// convertAPIToJob converts a Dice API job to model.JobPost.
func convertAPIToJob(j diceAPIJob) model.JobPost {
	title := strings.TrimSpace(j.Title)
	if title == "" {
		return model.JobPost{Title: ""}
	}

	// Build job URL
	jobURL := strings.TrimSpace(j.DetailsPageURL)
	if jobURL != "" && !strings.HasPrefix(jobURL, "http") {
		jobURL = "https://www.dice.com" + jobURL
	}
	if jobURL == "" && j.ID != "" {
		jobURL = "https://www.dice.com/job-detail/" + j.ID
	}
	if jobURL == "" && j.JobID != "" {
		jobURL = "https://www.dice.com/job-detail/" + j.JobID
	}

	// Build ID
	id := diceID(j.ID, j.JobID, jobURL)

	// Location
	loc := model.Location{}
	locationStr := ""
	if v := strings.TrimSpace(j.FormattedLocation); v != "" {
		locationStr = v
	} else if j.JobLocation != nil && strings.TrimSpace(j.JobLocation.DisplayName) != "" {
		locationStr = strings.TrimSpace(j.JobLocation.DisplayName)
	}
	if locationStr != "" {
		loc = parseLocation(locationStr)
	}

	// Remote detection
	isRemote := j.IsRemote || strings.Contains(strings.ToLower(locationStr), "remote")

	// Compensation
	var compensation *model.Compensation
	if j.PayRateRange != nil && (j.PayRateRange.Min != nil || j.PayRateRange.Max != nil) {
		compensation = &model.Compensation{
			Interval:  model.IntervalYearly,
			MinAmount: j.PayRateRange.Min,
			MaxAmount: j.PayRateRange.Max,
			Currency:  "USD",
		}
	} else if v := strings.TrimSpace(j.Salary); v != "" {
		if minVal, maxVal, ok := parseSalaryRange(v); ok {
			compensation = &model.Compensation{
				Interval:  model.IntervalYearly,
				MinAmount: &minVal,
				MaxAmount: &maxVal,
				Currency:  "USD",
			}
		}
	}

	// Date posted
	var datePosted *time.Time
	if v := strings.TrimSpace(j.PostedDate); v != "" {
		if t, err := parseDiceDate(v); err == nil {
			datePosted = &t
		}
	}
	if datePosted == nil {
		if v := strings.TrimSpace(j.ModifiedDate); v != "" {
			if t, err := parseDiceDate(v); err == nil {
				datePosted = &t
			}
		}
	}

	return model.JobPost{
		ID:          id,
		Title:       title,
		CompanyName: strings.TrimSpace(j.CompanyName),
		JobURL:      jobURL,
		Location:    loc,
		IsRemote:    isRemote,
		Description: strings.TrimSpace(j.Summary),
		JobType:     mapEmploymentType(j.EmploymentType),
		Compensation: compensation,
		DatePosted:  datePosted,
	}
}

// parseHTMLJobs extracts job postings from Dice HTML using regex.
func parseHTMLJobs(html []byte) []model.JobPost {
	raw := string(html)

	titles := findAll(titleRe, raw)
	companies := findAll(companyRe, raw)
	locations := findAll(locationRe, raw)
	salaries := findAll(salaryRe, raw)
	dates := findAll(dateRe, raw)
	urls := findAll(jobURLRe, raw)

	// Count job cards by data-cy="search-card" attributes.
	cardCount := len(cardRe.FindAllString(raw, -1))
	if cardCount == 0 {
		cardCount = len(titles)
	}
	if cardCount > pageSize {
		cardCount = pageSize
	}

	jobs := make([]model.JobPost, 0, cardCount)
	for i := 0; i < cardCount; i++ {
		title := ""
		if i < len(titles) {
			title = titles[i]
		}
		if title == "" {
			continue
		}

		jobURL := ""
		if i < len(urls) {
			href := urls[i]
			if strings.HasPrefix(href, "/") {
				jobURL = "https://www.dice.com" + href
			} else if !strings.HasPrefix(href, "http") {
				jobURL = "https://www.dice.com/" + href
			} else {
				jobURL = href
			}
		}

		id := diceID("", "", jobURL+title)
		loc := model.Location{}
		if i < len(locations) && locations[i] != "" {
			loc = parseLocation(locations[i])
		}

		isRemote := strings.Contains(strings.ToLower(loc.City), "remote") ||
			strings.Contains(strings.ToLower(loc.State), "remote")

		var compensation *model.Compensation
		if i < len(salaries) && salaries[i] != "" {
			if minVal, maxVal, ok := parseSalaryRange(salaries[i]); ok {
				compensation = &model.Compensation{
					Interval:  model.IntervalYearly,
					MinAmount: &minVal,
					MaxAmount: &maxVal,
					Currency:  "USD",
				}
			}
		}

		datePosted := parseDateText(dates, i)

		company := ""
		if i < len(companies) {
			company = companies[i]
		}

		job := model.JobPost{
			ID:           id,
			Title:        title,
			CompanyName:  company,
			Location:     loc,
			JobURL:       jobURL,
			DatePosted:   datePosted,
			IsRemote:     isRemote,
			Compensation: compensation,
		}
		jobs = append(jobs, job)
	}

	return jobs
}

// findAll extracts all submatch-1 values from the regex in document order.
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

// parseLocation splits a location string into City and State parts.
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

// parseDiceDate parses an ISO date string from the Dice API.
func parseDiceDate(v string) (time.Time, error) {
	// Try RFC3339 first
	if t, err := time.Parse(time.RFC3339, v); err == nil {
		return t.UTC(), nil
	}
	// Try ISO 8601 with possible variations
	formats := []string{
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05.000Z",
		"2006-01-02T15:04:05-07:00",
		"2006-01-02",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, v); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot parse date: %s", v)
}

// parseDateText parses relative date strings from HTML ("Posted 2 days ago", etc.).
func parseDateText(dates []string, i int) *time.Time {
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
	case strings.Contains(text, "day"):
		var days int
		if _, err := fmt.Sscanf(text, "%d", &days); err == nil && days > 0 {
			t := time.Date(now.Year(), now.Month(), now.Day()-days, 0, 0, 0, 0, now.Location())
			return &t
		}
	case strings.Contains(text, "week"):
		var weeks int
		if _, err := fmt.Sscanf(text, "%d", &weeks); err == nil && weeks > 0 {
			t := now.AddDate(0, 0, -weeks*7)
			return &t
		}
	case strings.Contains(text, "month"):
		var months int
		if _, err := fmt.Sscanf(text, "%d", &months); err == nil && months > 0 {
			t := now.AddDate(0, -months, 0)
			return &t
		}
	}
	return nil
}

// parseSalaryRange extracts min and max from a salary string like "$150,000 - $200,000".
func parseSalaryRange(s string) (float64, float64, bool) {
	matches := salaryRangeRe.FindStringSubmatch(s)
	if len(matches) < 3 {
		return 0, 0, false
	}
	minStr := strings.ReplaceAll(matches[1], ",", "")
	maxStr := strings.ReplaceAll(matches[2], ",", "")
	min, err1 := strconv.ParseFloat(minStr, 64)
	max, err2 := strconv.ParseFloat(maxStr, 64)
	if err1 != nil || err2 != nil || min <= 0 || max <= 0 {
		return 0, 0, false
	}
	return min, max, true
}

// mapEmploymentType maps Dice employmentType to our JobType string.
func mapEmploymentType(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	switch v {
	case "full-time", "fulltime":
		return "fulltime"
	case "part-time", "parttime":
		return "parttime"
	case "contract", "contractor":
		return "contract"
	case "internship":
		return "internship"
	case "temporary", "temp":
		return "temporary"
	default:
		return v
	}
}

// diceID generates a stable ID for a Dice job posting.
func diceID(id, jobID, url string) string {
	key := id
	if key == "" {
		key = jobID
	}
	if key == "" {
		key = url
	}
	return "dice-" + util.HashID(key)
}

// enrichWithFullDescriptions fetches full job descriptions from Dice detail pages
// for jobs whose Summary text is too short to contain email addresses.
// This is the key to making email extraction work on Dice.
func (s *Scraper) enrichWithFullDescriptions(ctx context.Context, jobs []model.JobPost) {
	const minDescLen = 100     // skip detail fetch if summary is already this long
	const maxFetches = 10      // cap detail page fetches to limit latency
	fetched := 0
	for i := range jobs {
		if ctx.Err() != nil {
			return
		}
		if len(jobs[i].Description) >= minDescLen {
			continue
		}
		if jobs[i].JobURL == "" {
			continue
		}
		if fetched >= maxFetches {
			break
		}
		fetched++
		desc := s.fetchDetailDescription(ctx, jobs[i].JobURL)
		if desc != "" {
			jobs[i].Description = util.StripHTML(desc)
		}
	}
	if fetched > 0 {
		util.Debug("dice_detail_fetched", map[string]any{"count": fetched, "total_jobs": len(jobs)})
	}
}

// fetchDetailDescription fetches a Dice job detail page and extracts the full description.
func (s *Scraper) fetchDetailDescription(ctx context.Context, jobURL string) string {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, jobURL, nil)
	if err != nil {
		return ""
	}
	for k, v := range htmlHeaders {
		req.Header.Set(k, v)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return ""
	}

	body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return ""
	}

	// Try JSON-LD first (most reliable)
	jsonldPosts := util.ExtractJobPostingsJSONLD(body)
	if len(jsonldPosts) > 0 && strings.TrimSpace(jsonldPosts[0].Description) != "" {
		return jsonldPosts[0].Description
	}

	// Fallback: extract from __NEXT_DATA__ (Next.js SSR)
	if desc := extractDescriptionFromNextData(body); desc != "" {
		return desc
	}

	// Final fallback: extract from HTML meta description or visible content
	if desc := extractDescriptionFromHTML(body); desc != "" {
		return desc
	}

	return ""
}

// extractDescriptionFromNextData parses Dice's Next.js __NEXT_DATA__ script tag
// to extract the full job description.
func extractDescriptionFromNextData(body []byte) string {
	m := reNextData.FindSubmatch(body)
	if len(m) < 2 {
		return ""
	}
	var parsed struct {
		Props struct {
			PageProps struct {
				Job struct {
					Description string `json:"description"`
					Body        string `json:"body"`
					Summary     string `json:"summary"`
				} `json:"job"`
			} `json:"pageProps"`
		} `json:"props"`
	}
	if err := json.Unmarshal(m[1], &parsed); err != nil {
		return ""
	}
	if v := strings.TrimSpace(parsed.Props.PageProps.Job.Description); v != "" && len(v) > 50 {
		return v
	}
	if v := strings.TrimSpace(parsed.Props.PageProps.Job.Body); v != "" && len(v) > 50 {
		return v
	}
	if v := strings.TrimSpace(parsed.Props.PageProps.Job.Summary); v != "" && len(v) > 100 {
		return v
	}
	return ""
}

// extractDescriptionFromHTML is a regex-based fallback that looks for
// description-like content in the Dice detail page HTML.
func extractDescriptionFromHTML(body []byte) string {
	s := string(body)

	// Dice often uses a <div> with data-testid or aria-label containing description
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?is)<div[^>]*(?:data-testid|aria-label)[^>]*job-description[^>]*>\s*<p>(.*?)</p>`),
		regexp.MustCompile(`(?is)<div[^>]*(?:data-testid|aria-label)[^>]*(?:description|Description)[^>]*>(.*?)</div>`),
		regexp.MustCompile(`(?is)<section[^>]*(?:description|Description)[^>]*>(.*?)</section>`),
		// Meta description fallback
		regexp.MustCompile(`(?is)<meta[^>]*name=["']description["'][^>]*content=["']([^"']+)["']`),
	}

	for _, re := range patterns {
		if m := re.FindStringSubmatch(s); len(m) > 1 {
			v := strings.TrimSpace(m[1])
			if len(v) > 50 {
				tagRe := regexp.MustCompile(`<[^>]+>`)
				v = tagRe.ReplaceAllString(v, " ")
				v = strings.Join(strings.Fields(v), " ")
				return v
			}
		}
	}
	return ""
}


