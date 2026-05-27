package monster

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/arinbalyan/scrappy/internal/browser"
	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/util"
)

const (
	searchURL = "https://www.monster.com/jobs/search"
	pageSize  = 20
	maxPages  = 100
)

// Browser-like headers to reduce bot detection (from monster.constants.ts).
var defaultHeaders = map[string]string{
	"Accept":                    "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7",
	"Accept-Language":           "en-US,en;q=0.9",
	"User-Agent":                "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36",
	"Cache-Control":             "no-cache",
	"Sec-Fetch-Dest":            "document",
	"Sec-Fetch-Mode":            "navigate",
	"Sec-Fetch-Site":            "none",
	"Sec-Fetch-User":            "?1",
	"Upgrade-Insecure-Requests": "1",
}

// Field-level regex patterns matching Monster's HTML structure.
// Monster job listings use data-testid attributes that are stable across pages.
var (
	// Primary selectors (data-testid attributes)
	titleRe    = regexp.MustCompile(`data-testid="jobTitle"[^>]*>\s*([^<]+)\s*<`)
	companyRe  = regexp.MustCompile(`data-testid="company"[^>]*>\s*([^<]+)\s*<`)
	locationRe = regexp.MustCompile(`data-testid="jobLocation"[^>]*>\s*([^<]+)\s*<`)
	salaryRe   = regexp.MustCompile(`data-testid="svx_jobCard-salary"[^>]*>\s*([^<]+)\s*<`)

	// URL extraction — href on links within or near job cards
	hrefRe = regexp.MustCompile(`<a[^>]*href="(/(?:jobs|job|job-view)/[^"]+)"`)

	// Date extraction — ISO date from <time datetime="...">
	dateRe = regexp.MustCompile(`<time[^>]*datetime="([^"]+)"`)

	// Fallback selectors (CSS class patterns for when data-testid is absent)
	fallbackTitleRe    = regexp.MustCompile(`class="[^"]*job-cardstyle__JobCardTitle[^"]*"[^>]*>\s*<a[^>]*>\s*([^<]+)\s*<`)
	fallbackCompanyRe  = regexp.MustCompile(`class="[^"]*job-cardstyle__JobCardCompany[^"]*"[^>]*>\s*([^<]+)\s*<`)
	fallbackLocationRe = regexp.MustCompile(`class="[^"]*job-cardstyle__JobCardLocation[^"]*"[^>]*>\s*([^<]+)\s*<`)
	fallbackSalaryRe   = regexp.MustCompile(`class="[^"]*job-cardstyle__JobCardSalary[^"]*"[^>]*>\s*([^<]+)\s*<`)
	fallbackDateRe     = regexp.MustCompile(`class="[^"]*job-cardstyle__JobCardDate[^"]*"[^>]*>\s*([^<]+)\s*<`)
)

// Scraper implements the scraper.Scraper interface for Monster.
type Scraper struct {
	client    *http.Client
	searchURL string
}

// New creates a new Monster scraper. If client is nil, a default HTTP client
// with retries and timeout is created.
func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Retries: 2, Timeout: 30 * time.Second})
	}
	return &Scraper{client: client, searchURL: searchURL}
}

// NewWithURLs creates a scraper with a custom search URL override for testing.
func NewWithURLs(client *http.Client, endpoint string) *Scraper {
	s := New(client)
	if strings.TrimSpace(endpoint) != "" {
		s.searchURL = strings.TrimSpace(endpoint)
	}
	return s
}

// SiteName returns model.SiteMonster.
func (s *Scraper) SiteName() model.Site { return model.SiteMonster }

// Scrape fetches job listings from Monster's HTML search pages.
// It handles page-based pagination via page parameter, respects context
// cancellation, and rate-limits to ~3 requests/second.
func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	wanted := input.ResultsWanted
	if wanted <= 0 {
		wanted = 15
	}

	searchTerm := strings.TrimSpace(input.SearchTerm)
	if searchTerm == "" {
		return nil, fmt.Errorf("monster: search term required")
	}

	// Parse OR terms — use the first term for server-side URL, filter all via client-side matchAny
	terms := parseSearchTerms(searchTerm)
	serverTerm := searchTerm
	if len(terms) > 0 {
		serverTerm = terms[0]
	}

	jobs := make([]model.JobPost, 0, wanted)
	page := 1
	maxPagesToFetch := maxPages
	if wanted > 0 {
		pagesNeeded := (wanted + pageSize - 1) / pageSize
		if pagesNeeded+1 < maxPagesToFetch {
			maxPagesToFetch = pagesNeeded + 1
		}
	}

	// Rate limiter: max 3 requests per second (~333ms minimum interval).
	rateLimiter := time.NewTicker(350 * time.Millisecond)
	defer rateLimiter.Stop()

	seen := make(map[string]struct{})

	for len(jobs) < wanted && page <= maxPagesToFetch {
		select {
		case <-ctx.Done():
			return jobs, ctx.Err()
		default:
		}

		body, err := s.fetchPage(ctx, serverTerm, input.Location, page)
		if err != nil {
			return nil, fmt.Errorf("monster page %d: %w", page, err)
		}

		pageJobs := parseJobs(body)
		if len(pageJobs) == 0 {
			// Monster may have changed its HTML — log snippet for debugging
			snippet := string(body)
			if len(snippet) > 500 {
				snippet = snippet[:500]
			}
			util.Debug("monster_no_jobs_html", map[string]any{
				"page":      page,
				"body_len":  len(body),
				"preview":   snippet,
			})
			break
		}

		for _, j := range pageJobs {
			if j.ID == "" || j.Title == "" {
				continue
			}
			// Client-side OR filtering across all terms
			if len(terms) > 0 {
				hay := strings.ToLower(j.Title + " " + j.Description)
				if !matchAny(hay, terms) {
					continue
				}
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
		return nil, fmt.Errorf("monster no parseable jobs")
	}
	if len(jobs) > wanted {
		jobs = jobs[:wanted]
	}
	return jobs, nil
}

// fetchPage makes one HTTP request to a Monster search page.
func (s *Scraper) fetchPage(ctx context.Context, searchTerm, location string, page int) ([]byte, error) {
	u, err := url.Parse(s.searchURL)
	if err != nil {
		return nil, fmt.Errorf("monster url: %w", err)
	}

	q := u.Query()
	q.Set("q", searchTerm)
	if strings.TrimSpace(location) != "" {
		q.Set("where", strings.TrimSpace(location))
	}
	q.Set("page", fmt.Sprintf("%d", page))
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	setDefaultHeaders(req)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("monster request: %w", err)
	}
	defer resp.Body.Close()

	body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("monster read: %w", err)
	}

	// Check for anti-bot / WAF challenge page even on non-2xx responses
	// (DataDome / Cloudflare often return 403 with a challenge body).
	if challenge := util.DetectAntiBotChallenge(body); challenge != "" {
		// Try browser fallback if available.
		if browser.IsAvailable() {
			browserCtx, cancel := context.WithTimeout(ctx, 35*time.Second)
			defer cancel()
			result, bErr := browser.FetchPage(browserCtx, u.String(), "data-testid")
			if bErr == nil && result.Status == 200 && len(result.HTML) > 0 {
				return []byte(result.HTML), nil
			}
		}
		return nil, fmt.Errorf("blocked - %s challenge detected — try using --proxy with a residential proxy", challenge)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("monster status %d — try using --proxy with a residential proxy", resp.StatusCode)
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

// parseJobs extracts job postings from Monster HTML search results.
// It uses ordered field extraction: finds all titles, companies, locations,
// URLs, and dates in the order they appear, then zips them into JobPost records.
// Multiple selector strategies are tried with fallbacks.
// parseSearchTerms splits a search term on " OR " and returns lowercase terms.
func parseSearchTerms(raw string) []string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return nil
	}
	parts := strings.Split(s, " OR ")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.Trim(strings.TrimSpace(p), `"`)
		if p != "" {
			out = append(out, strings.ToLower(p))
		}
	}
	return out
}

// matchAny returns true if the haystack contains any of the terms.
func matchAny(hay string, terms []string) bool {
	for _, t := range terms {
		if strings.Contains(hay, t) {
			return true
		}
	}
	return false
}

func parseJobs(html []byte) []model.JobPost {
	raw := string(html)

	// Try primary data-testid selectors first, then fallback CSS class selectors.
	titles := extractAll(titleRe, raw)
	if len(titles) == 0 {
		titles = extractAll(fallbackTitleRe, raw)
	}

	companies := extractAll(companyRe, raw)
	if len(companies) == 0 {
		companies = extractAll(fallbackCompanyRe, raw)
	}

	locations := extractAll(locationRe, raw)
	if len(locations) == 0 {
		locations = extractAll(fallbackLocationRe, raw)
	}

	salaries := extractAll(salaryRe, raw)
	if len(salaries) == 0 {
		salaries = extractAll(fallbackSalaryRe, raw)
	}

	dates := extractAllDates(raw)

	// For URLs, extract ALL href="/jobs/..." patterns (not just first match).
	hrefs := extractHrefs(raw)

	// Determine card count from the number of titles or hrefs.
	cardCount := len(titles)
	if cardCount == 0 {
		cardCount = len(hrefs)
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

		company := ""
		if i < len(companies) {
			company = companies[i]
		}

		jobURL := ""
		if i < len(hrefs) {
			href := hrefs[i]
			if strings.HasPrefix(href, "/") {
				jobURL = "https://www.monster.com" + href
			} else if !strings.HasPrefix(href, "http") {
				jobURL = "https://www.monster.com/" + href
			} else {
				jobURL = href
			}
		}

		id := "monster-" + util.HashID(jobURL + title + company)

		loc := model.Location{}
		if i < len(locations) && locations[i] != "" {
			loc = parseLocation(locations[i])
		}

		isRemote := strings.Contains(strings.ToLower(loc.City), "remote") ||
			strings.Contains(strings.ToLower(loc.State), "remote")

		datePosted := parseDate(dates, i)

		job := model.JobPost{
			ID:           id,
			Title:        title,
			CompanyName:  company,
			Location:     loc,
			JobURL:       jobURL,
			DatePosted:   datePosted,
			IsRemote:     isRemote,
			ApplyMethod:  "external_url",
		}
		jobs = append(jobs, job)
	}

	return jobs
}

// extractAll extracts all submatch-1 values from the regex applied to html,
// in document order.
func extractAll(re *regexp.Regexp, html string) []string {
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

// extractAllDates extracts dates from <time datetime="..."> elements and
// also falls back to CSS-class-based date text.
func extractAllDates(raw string) []string {
	dates := extractAll(dateRe, raw)
	if len(dates) == 0 {
		dates = extractAll(fallbackDateRe, raw)
	}
	return dates
}

// extractHrefs extracts all job-related href values from Monster HTML.
func extractHrefs(raw string) []string {
	matches := hrefRe.FindAllStringSubmatch(raw, -1)
	out := make([]string, 0, len(matches))
	seen := make(map[string]struct{}, len(matches))
	for _, m := range matches {
		if len(m) >= 2 {
			v := strings.TrimSpace(m[1])
			if v == "" {
				continue
			}
			if _, exists := seen[v]; exists {
				continue
			}
			seen[v] = struct{}{}
			out = append(out, v)
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

// parseDate extracts a date from Monster's date formats.
// Monster uses ISO datetime attributes on <time> elements.
func parseDate(dates []string, i int) *time.Time {
	if i >= len(dates) || dates[i] == "" {
		return nil
	}
	return util.ParseDatePosted(dates[i])
}


