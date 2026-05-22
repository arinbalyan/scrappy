package careerbuilder

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
	searchURL = "https://www.careerbuilder.com/jobs"
	pageSize  = 25
	maxPages  = 100
)

// Browser-like headers to reduce bot detection (from careerbuilder.constants.ts).
var defaultHeaders = map[string]string{
	"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7",
	"Accept-Language": "en-US,en;q=0.9",
	"Accept-Encoding": "gzip, deflate, br",
	"User-Agent":      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36",
	"Cache-Control":   "no-cache",
	"Sec-Fetch-Dest":  "document",
	"Sec-Fetch-Mode":  "navigate",
	"Sec-Fetch-Site":  "none",
}

// Field-level regex patterns matching CareerBuilder's HTML structure.
// CareerBuilder job listings use data-* attributes and CSS class names
// that are stable across pages.
var (
	titleRe      = regexp.MustCompile(`data-results-title[^>]*>\s*<a[^>]*>\s*([^<]+)\s*<`)
	companyRe    = regexp.MustCompile(`data-company[^>]*>\s*([^<]+)\s*<`)
	locationRe   = regexp.MustCompile(`data-location[^>]*>\s*([^<]+)\s*<`)
	jobURLRe     = regexp.MustCompile(`<a[^>]*href="(/jobs/[^"]+)"`)
	dateRe       = regexp.MustCompile(`data-posted-date[^>]*>\s*([^<]+)\s*<`)
	didCountRe   = regexp.MustCompile(`data-job-did="`)
)

// Scraper implements the scraper.Scraper interface for CareerBuilder.
type Scraper struct {
	client    *http.Client
	searchURL string
}

// New creates a new CareerBuilder scraper. If client is nil, a default
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

// SiteName returns model.SiteCareerBuilder.
func (s *Scraper) SiteName() model.Site { return model.SiteCareerBuilder }

// Scrape fetches job listings from CareerBuilder's HTML search pages.
// It handles page-based pagination via page_number parameter, respects
// context cancellation, and rate-limits to ~2 requests/second.
func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	wanted := input.ResultsWanted
	if wanted <= 0 {
		wanted = 15
	}

	searchTerm := strings.TrimSpace(input.SearchTerm)
	if searchTerm == "" {
		return nil, fmt.Errorf("careerbuilder: search term required")
	}

	jobs := make([]model.JobPost, 0, wanted)
	page := 1
	maxPagesToFetch := maxPages
	if wanted > 0 {
		maxPagesToFetch = min(maxPages, (wanted+pageSize-1)/pageSize+1)
	}

	// Rate limiter: max 2 requests per second (500ms minimum interval).
	rateLimiter := time.NewTicker(500 * time.Millisecond)
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
			return nil, fmt.Errorf("careerbuilder page %d: %w", page, err)
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
		return nil, fmt.Errorf("careerbuilder no parseable jobs")
	}
	if len(jobs) > wanted {
		jobs = jobs[:wanted]
	}
	return jobs, nil
}

// fetchPage makes one HTTP request to a CareerBuilder search page.
func (s *Scraper) fetchPage(ctx context.Context, searchTerm, location string, page int) ([]byte, error) {
	u, err := url.Parse(s.searchURL)
	if err != nil {
		return nil, fmt.Errorf("careerbuilder url: %w", err)
	}

	q := u.Query()
	q.Set("keywords", searchTerm)
	if strings.TrimSpace(location) != "" {
		q.Set("location", strings.TrimSpace(location))
	}
	q.Set("page_number", fmt.Sprintf("%d", page))
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	setDefaultHeaders(req)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("careerbuilder request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("careerbuilder status %d", resp.StatusCode)
	}

	body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("careerbuilder read: %w", err)
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

// parseJobs extracts job postings from CareerBuilder HTML.
// It uses ordered field extraction: finds all titles, companies, locations, URLs,
// and dates in the order they appear, then zips them into JobPost records.
// This mirrors how the TypeScript cheerio parser processes cards sequentially.
func parseJobs(html []byte) []model.JobPost {
	raw := string(html)

	titles := findAll(titleRe, raw)
	companies := findAll(companyRe, raw)
	locations := findAll(locationRe, raw)
	urls := findAll(jobURLRe, raw)
	dates := findAll(dateRe, raw)

	// Count actual job cards by data-job-did attribute occurrences.
	cardCount := len(didCountRe.FindAllString(raw, -1))
	if cardCount == 0 {
		cardCount = len(titles)
	}
	if cardCount > 25 {
		cardCount = 25
	}

	jobs := make([]model.JobPost, 0, cardCount)
	for i := 0; i < cardCount; i++ {
		title := ""
		if i < len(titles) {
			title = titles[i]
		}
		company := ""
		if i < len(companies) {
			company = companies[i]
		}
		if title == "" {
			continue
		}

		jobURL := ""
		if i < len(urls) {
			href := urls[i]
			if strings.HasPrefix(href, "/") {
				jobURL = "https://www.careerbuilder.com" + href
			} else if !strings.HasPrefix(href, "http") {
				jobURL = "https://www.careerbuilder.com/" + href
			} else {
				jobURL = href
			}
		}

		id := "cb-" + hashID(jobURL+title+company)
		loc := model.Location{}
		if i < len(locations) && locations[i] != "" {
			loc = parseLocation(locations[i])
		}

		datePosted := parseDate(dates, i)
		isRemote := strings.Contains(strings.ToLower(loc.City), "remote") ||
			strings.Contains(strings.ToLower(loc.State), "remote")

		job := model.JobPost{
			ID:          id,
			Title:       title,
			CompanyName: company,
			Location:    loc,
			JobURL:      jobURL,
			DatePosted:  datePosted,
			IsRemote:    isRemote,
		}
		jobs = append(jobs, job)
	}

	return jobs
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

// parseLocation splits a "City, State" or "City, State, Country" location string.
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

// parseDate extracts a date from CareerBuilder's posted-date format.
// Common formats: "Posted 2 days ago", "30+ days ago", "Posted Today".
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
	case strings.Contains(text, "day"):
		// Extract number of days from patterns like "2 days ago" or "30+ days ago"
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
	default:
		// Try parsing as an ISO date
		if t, err := time.Parse("2006-01-02", text); err == nil {
			return &t
		}
	}
	return nil
}

// hashID generates a stable hash string for deduplication.
func hashID(s string) string {
	h := fnv.New64a()
	h.Write([]byte(s))
	return fmt.Sprintf("%d", h.Sum64())
}
