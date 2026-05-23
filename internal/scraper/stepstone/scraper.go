// Package stepstone implements a StepStone (stepstone.com) HTML job scraper.
// StepStone is a major European job board, primarily serving Germany, Austria,
// Belgium, and the Netherlands. The scraper extracts job listings from search
// result pages using data-testid / data-at attribute patterns found in the
// server-rendered React HTML. It also enriches results via JSON-LD when available.
package stepstone

import (
	"context"
	"encoding/json"
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

// Domain defaults and per-country domains for StepStone.
const defaultDomain = "www.stepstone.de"

// stepstoneDomains maps country codes to their StepStone domains.
var stepstoneDomains = map[string]string{
	"germany":     "www.stepstone.de",
	"austria":     "www.stepstone.at",
	"belgium":     "www.stepstone.be",
	"netherlands": "www.stepstone.nl",
}

// Browser-like headers matching a modern Chrome on macOS to reduce bot detection.
var defaultHeaders = map[string]string{
	"Accept":                    "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7",
	"Accept-Language":           "en-US,en;q=0.9",
	"Accept-Encoding":           "gzip, deflate, br",
	"User-Agent":                "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36",
	"Cache-Control":             "no-cache",
	"Sec-Fetch-Dest":            "document",
	"Sec-Fetch-Mode":            "navigate",
	"Sec-Fetch-Site":            "none",
	"Sec-Fetch-User":            "?1",
	"Upgrade-Insecure-Requests": "1",
}

// Regex patterns for extracting job data from StepStone's HTML.
//
// StepStone uses data-testid and data-at attributes on server-rendered job
// result cards. These patterns follow the same structure as Monster's HTML
// scraper — primary attribute-based selectors with CSS-class fallbacks.
var (
	// Card-level detection: match article elements with known StepStone attributes.
	// These are the primary container selectors from the TypeScript source.
	cardDataTestIDRe = regexp.MustCompile(`<article[^>]*data-testid="job-item"[^>]*>[\s\S]*?</article>`)
	cardDataAtRe     = regexp.MustCompile(`<[^>]+data-at="job-item"[^>]*>[\s\S]*?</[a-z]+>`)

	// Title extraction: data-at attribute on heading links.
	titleRe = regexp.MustCompile(`data-at="job-item-title"[^>]*>\s*([^<]+)\s*<`)

	// Company name extraction.
	companyRe          = regexp.MustCompile(`data-at="job-item-company-name"[^>]*>\s*([^<]+)\s*<`)
	fallbackCompanyRe  = regexp.MustCompile(`class="[^"]*res-company-name[^"]*"[^>]*>\s*([^<]+)\s*<`)

	// Location extraction.
	locationRe          = regexp.MustCompile(`data-at="job-item-location"[^>]*>\s*([^<]+)\s*<`)
	fallbackLocationRe  = regexp.MustCompile(`class="[^"]*res-location[^"]*"[^>]*>\s*([^<]+)\s*<`)

	// URL extraction — StepStone job detail URLs use /stellenangebote-- prefix.
	hrefRe = regexp.MustCompile(`<a[^>]*href="(/stellenangebote--[^"]+)"`)

	// JSON-LD extraction for job detail enrichment (description, salary).
	jsonLdRe = regexp.MustCompile(`<script type="application/ld\+json">\s*(\{[\s\S]*?\})\s*</script>`)
)

// StepStoneJsonLd maps the JSON-LD JobPosting schema used on StepStone detail pages.
type StepStoneJsonLd struct {
	Type              string       `json:"@type"`
	Title             string       `json:"title"`
	Description       string       `json:"description"`
	DatePosted        string       `json:"datePosted"`
	EmploymentType    string       `json:"employmentType"`
	HiringOrganization *struct {
		Name string `json:"name"`
	} `json:"hiringOrganization,omitempty"`
	JobLocation *struct {
		Address *struct {
			AddressLocality  string `json:"addressLocality"`
			AddressRegion    string `json:"addressRegion"`
			AddressCountry   string `json:"addressCountry"`
		} `json:"address,omitempty"`
	} `json:"jobLocation,omitempty"`
	BaseSalary *struct {
		Currency string `json:"currency"`
		Value    *struct {
			MinValue float64 `json:"minValue"`
			MaxValue float64 `json:"maxValue"`
		} `json:"value,omitempty"`
	} `json:"baseSalary,omitempty"`
}

// Scraper implements the scraper.Scraper interface for StepStone.
type Scraper struct {
	client  *http.Client
	domain  string
	testURL string // when set (testing), use as-is instead of constructing URL
}

// New creates a new StepStone scraper. If client is nil, a default HTTP client
// with retries and timeout is created. The default domain is www.stepstone.de.
func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Retries: 2, Timeout: 30 * time.Second})
	}
	return &Scraper{client: client, domain: defaultDomain}
}

// NewWithURLs creates a scraper with a custom endpoint override for testing.
// The endpoint replaces the full search URL when set, allowing tests to point
// at an httptest.Server.
func NewWithURLs(client *http.Client, endpoint string) *Scraper {
	s := New(client)
	if strings.TrimSpace(endpoint) != "" {
		s.testURL = strings.TrimSpace(endpoint)
	}
	return s
}

// SiteName returns model.SiteStepStone.
func (s *Scraper) SiteName() model.Site { return model.SiteStepStone }

// Scrape fetches job listings from StepStone's HTML search results page.
// It constructs the search URL, fetches the page, parses job cards using regex,
// and enriches results with JSON-LD data when available. This scraper targets
// the server-rendered HTML that StepStone's React app produces.
//
// Rate limiting: ~3 requests/second (one single page fetch per scrape call).
func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	wanted := input.ResultsWanted
	if wanted <= 0 {
		wanted = 15
	}

	searchTerm := strings.TrimSpace(input.SearchTerm)
	if searchTerm == "" {
		searchTerm = "developer"
	}

	// Build or use the search URL.
	var searchURL string
	if s.testURL != "" {
		searchURL = s.testURL
	} else {
		domain := s.domain
		if v, ok := stepstoneDomains[strings.ToLower(strings.TrimSpace(string(input.Country)))]; ok {
			domain = v
		}
		slug := util.NormalizeSlug(searchTerm)
		searchURL = fmt.Sprintf("https://%s/jobs/%s", domain, url.PathEscape(slug))
	}

	body, err := s.fetchPage(ctx, searchURL)
	if err != nil {
		return nil, fmt.Errorf("stepstone: %w", err)
	}

	html := string(body)

	// Parse job cards from HTML.
	jobs := s.parseJobCards(html, searchURL)

	// Enrich with JSON-LD data if present.
	s.enrichFromJSONLD(html, jobs)

	if !util.HasMeaningfulJobs(jobs) {
		return nil, fmt.Errorf("stepstone: no parseable jobs found")
	}
	if len(jobs) > wanted {
		jobs = jobs[:wanted]
	}
	return jobs, nil
}

// fetchPage makes one HTTP request to a StepStone search page.
func (s *Scraper) fetchPage(ctx context.Context, pageURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	for k, v := range defaultHeaders {
		req.Header.Set(k, v)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("http status %d", resp.StatusCode)
	}

	body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("read: %w", err)
	}
	return body, nil
}

// parseJobCards extracts job postings from StepStone search result HTML.
// It tries multiple card-level selectors and zips field-level extractions
// by document position, matching the approach used by the Monster scraper.
func (s *Scraper) parseJobCards(html string, baseURL string) []model.JobPost {
	// Try to extract job cards by block.
	cardBlocks := s.extractCardBlocks(html)

	if len(cardBlocks) > 0 {
		return s.parseCardBlocks(cardBlocks, baseURL)
	}

	// Fallback: field-level extraction by document position.
	return s.parseFieldLevel(html, baseURL)
}

// extractCardBlocks attempts to find individual job card HTML blocks using
// known StepStone container selectors.
func (s *Scraper) extractCardBlocks(html string) []string {
	// Try data-testid="job-item" articles first (primary selector).
	matches := cardDataTestIDRe.FindAllString(html, -1)
	if len(matches) > 0 {
		return matches
	}

	// Fall back to data-at="job-item" containers.
	matches = cardDataAtRe.FindAllString(html, -1)
	if len(matches) > 0 {
		return matches
	}

	return nil
}

// parseCardBlocks extracts job data from individually identified card blocks.
func (s *Scraper) parseCardBlocks(blocks []string, baseURL string) []model.JobPost {
	out := make([]model.JobPost, 0, len(blocks))
	seen := make(map[string]struct{})

	for _, block := range blocks {
		title := extractField(block, titleRe, "")
		if title == "" {
			continue
		}

		company := extractField(block, companyRe, "")
		if company == "" {
			company = extractField(block, fallbackCompanyRe, "")
		}

		location := extractField(block, locationRe, "")
		if location == "" {
			location = extractField(block, fallbackLocationRe, "")
		}

		href := extractField(block, hrefRe, "")
		jobURL := s.resolveURL(href, baseURL)
		if jobURL == "" {
			continue
		}

		id := "st-" + hashID(jobURL)
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}

		loc := parseLocationString(location)
		job := model.JobPost{
			ID:          id,
			Title:       title,
			CompanyName: company,
			JobURL:      jobURL,
			Location:    loc,
		}
		out = append(out, job)
	}

	return out
}

// parseFieldLevel extracts job fields from the full HTML by zipping field-level
// regex matches by document position. Used as fallback when card blocks can't
// be individually identified.
func (s *Scraper) parseFieldLevel(html string, baseURL string) []model.JobPost {
	titles := titleRe.FindAllStringSubmatch(html, -1)
	if len(titles) == 0 {
		return nil
	}

	companies := companyRe.FindAllStringSubmatch(html, -1)
	if len(companies) == 0 {
		companies = fallbackCompanyRe.FindAllStringSubmatch(html, -1)
	}

	locations := locationRe.FindAllStringSubmatch(html, -1)
	if len(locations) == 0 {
		locations = fallbackLocationRe.FindAllStringSubmatch(html, -1)
	}

	hrefs := hrefRe.FindAllStringSubmatch(html, -1)

	out := make([]model.JobPost, 0, len(titles))
	seen := make(map[string]struct{})

	for i, m := range titles {
		title := strings.TrimSpace(m[1])
		if title == "" || strings.Contains(title, "http") || strings.HasPrefix(title, "function") {
			continue
		}

		var company string
		if i < len(companies) {
			company = strings.TrimSpace(companies[i][1])
		}

		var location string
		if i < len(locations) {
			location = strings.TrimSpace(locations[i][1])
		}

		var jobURL string
		if i < len(hrefs) {
			jobURL = s.resolveURL(strings.TrimSpace(hrefs[i][1]), baseURL)
		}
		if jobURL == "" {
			jobURL = baseURL
		}

		id := "st-" + hashID(jobURL)
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}

		loc := parseLocationString(location)
		job := model.JobPost{
			ID:          id,
			Title:       title,
			CompanyName: company,
			JobURL:      jobURL,
			Location:    loc,
		}
		out = append(out, job)
	}

	return out
}

// enrichFromJSONLD parses JSON-LD script tags from the page and enriches
// job postings with description, salary, and other structured data.
func (s *Scraper) enrichFromJSONLD(html string, jobs []model.JobPost) {
	matches := jsonLdRe.FindAllStringSubmatch(html, -1)
	if len(matches) == 0 {
		return
	}

	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		var ld StepStoneJsonLd
		if err := json.Unmarshal([]byte(m[1]), &ld); err != nil {
			continue
		}
		if ld.Type != "JobPosting" || ld.Title == "" {
			continue
		}

		// Match by title to the closest job.
		for i := range jobs {
			if jobs[i].Title != ld.Title {
				continue
			}
			if ld.Description != "" && jobs[i].Description == "" {
				jobs[i].Description = stripHTML(ld.Description)
			}
			if ld.DatePosted != "" && jobs[i].DatePosted == nil {
				jobs[i].DatePosted = util.ParseDatePosted(ld.DatePosted)
			}
			if ld.BaseSalary != nil && ld.BaseSalary.Value != nil && jobs[i].Compensation == nil {
				interval := model.IntervalYearly
				currency := ld.BaseSalary.Currency
				if currency == "" {
					currency = "EUR"
				}
				minV := ld.BaseSalary.Value.MinValue
				maxV := ld.BaseSalary.Value.MaxValue
				jobs[i].Compensation = &model.Compensation{
					Interval: interval,
					MinAmount: &minV,
					MaxAmount: &maxV,
					Currency:  currency,
				}
			}
			if ld.HiringOrganization != nil && strings.TrimSpace(ld.HiringOrganization.Name) != "" && jobs[i].CompanyName == "" {
				jobs[i].CompanyName = strings.TrimSpace(ld.HiringOrganization.Name)
			}
			break
		}
	}
}

// resolveURL converts a relative href to an absolute URL.
func (s *Scraper) resolveURL(href, baseURL string) string {
	href = strings.TrimSpace(href)
	if href == "" {
		return ""
	}
	if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") {
		return href
	}
	if strings.HasPrefix(href, "/") {
		if strings.Contains(baseURL, "stepstone") {
			domain := s.domain
			if u, err := url.Parse(baseURL); err == nil {
				domain = u.Host
			}
			return fmt.Sprintf("https://%s%s", domain, href)
		}
		return strings.TrimRight(baseURL, "/") + href
	}
	return href
}

// extractField extracts the first match of a regex submatch from text.
func extractField(text string, re *regexp.Regexp, fallback string) string {
	matches := re.FindStringSubmatch(text)
	if len(matches) >= 2 {
		v := strings.TrimSpace(matches[1])
		if v != "" {
			return v
		}
	}
	return fallback
}

// parseLocationString splits a StepStone location string into City and State.
// StepStone locations are typically "City, Region" or "City" or "City, Region, Country".
func parseLocationString(location string) model.Location {
	location = strings.TrimSpace(location)
	if location == "" {
		return model.Location{}
	}
	parts := strings.SplitN(location, ",", 3)
	loc := model.Location{}
	if len(parts) >= 1 {
		loc.City = strings.TrimSpace(parts[0])
	}
	if len(parts) >= 2 {
		loc.State = strings.TrimSpace(parts[1])
	}
	if len(parts) >= 3 {
		loc.Country = strings.TrimSpace(parts[2])
	}
	return loc
}

// stripHTML removes basic HTML tags from a string (for description cleanup).
func stripHTML(s string) string {
	// Remove <tag> and </tag>
	re := regexp.MustCompile(`<[^>]*>`)
	s = re.ReplaceAllString(s, " ")
	// Collapse whitespace.
	re = regexp.MustCompile(`\s+`)
	s = re.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

// hashID computes a stable hash for a string (used for job ID generation).
func hashID(s string) string {
	h := fnv.New64a()
	h.Write([]byte(s))
	return fmt.Sprintf("%d", h.Sum64())
}
