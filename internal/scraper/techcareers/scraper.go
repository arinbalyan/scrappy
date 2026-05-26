package techcareers

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/util"
)

const (
	searchURL = "https://www.techcareers.com/jobs"
	maxPages  = 10
)

var (
	// jobCardRe matches individual job listing <article> blocks.
	// TechCareers uses <article> elements with "job" in the class.
	jobCardRe = regexp.MustCompile(`(?is)<article[^>]*class="[^"]*\bjob\b[^"]*"[^>]*>(.*?)</article>`)

	// extractByClass extracts text content from an element with a specific class.
	extractByClass = func(html, className string) string {
		re := regexp.MustCompile(fmt.Sprintf(`(?is)<[^>]+class="[^"]*\b%s\b[^"]*"[^>]*>([\s\S]*?)</[^>]+>`, regexp.QuoteMeta(className)))
		m := re.FindStringSubmatch(html)
		if len(m) < 2 {
			return ""
		}
		return strings.TrimSpace(stripTags(m[1]))
	}

	// extractLink extracts the text of the first anchor tag.
	extractLink = func(html string) string {
		re := regexp.MustCompile(`(?is)<a[^>]*>([^<]+)</a>`)
		m := re.FindStringSubmatch(html)
		if len(m) < 2 {
			return ""
		}
		return strings.TrimSpace(m[1])
	}

	// extractHref extracts the href of the first anchor tag.
	extractHref = func(html string) string {
		re := regexp.MustCompile(`(?is)<a[^>]+href="([^"]+)"`)
		m := re.FindStringSubmatch(html)
		if len(m) < 2 {
			return ""
		}
		return m[1]
	}

	stripTagsRe = regexp.MustCompile(`(?is)<[^>]+>`)
)

// Scraper fetches jobs from the TechCareers website.
type Scraper struct {
	client  *http.Client
	baseURL string
}

// New creates a new TechCareers scraper.
func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{
			Retries: 2,
			Timeout: 20 * time.Second,
			UserAgents: []string{
				"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/129.0.0.0 Safari/537.36",
			},
		})
	}
	return &Scraper{client: client, baseURL: searchURL}
}

// NewWithBaseURL creates a new scraper with a custom endpoint (used in tests).
func NewWithBaseURL(client *http.Client, baseURL string) *Scraper {
	s := New(client)
	if strings.TrimSpace(baseURL) != "" {
		s.baseURL = strings.TrimSpace(baseURL)
	}
	return s
}

// SiteName returns the site identifier.
func (s *Scraper) SiteName() model.Site { return model.SiteTechCareers }

// Scrape fetches jobs from TechCareers.
func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{
		"site":           s.SiteName(),
		"results_wanted": input.ResultsWanted,
		"search_term":    input.SearchTerm,
		"location":       input.Location,
	})

	wanted := input.ResultsWanted
	if wanted <= 0 {
		wanted = 25
	}

	term := strings.TrimSpace(input.SearchTerm)
	location := strings.TrimSpace(input.Location)

	jobs := make([]model.JobPost, 0, wanted)

	for page := 1; page <= maxPages && len(jobs) < wanted; page++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		url := s.baseURL
		q := urlQueryParams(term, location, page)
		if q != "" {
			url = s.baseURL + "?" + q
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("techcareers: build request: %w", err)
		}
		req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9")
		req.Header.Set("Accept-Language", "en-US,en;q=0.9")
		req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/129.0.0.0 Safari/537.36")

		resp, err := s.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("techcareers: request: %w", err)
		}

		body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("techcareers: read: %w", err)
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("techcareers: status %d", resp.StatusCode)
		}

		html := string(body)
		if len(html) < 100 {
			break
		}

		items := parseJobListings(html)
		util.Debug("techcareers: page", map[string]any{"page": page, "items": len(items)})

		if len(items) == 0 {
			break
		}

		for _, item := range items {
			if len(jobs) >= wanted {
				break
			}

			job, err := mapJob(item, term)
			if err != nil {
				continue
			}
			jobs = append(jobs, job)
		}
	}

	if !util.HasMeaningfulJobs(jobs) {
		return nil, fmt.Errorf("techcareers: no parseable jobs")
	}

	util.Debug("scraper_done", map[string]any{
		"site": s.SiteName(),
		"jobs": len(jobs),
	})
	return jobs, nil
}

// parsedJob holds fields extracted from an HTML job card.
type parsedJob struct {
	title       string
	url         string
	company     string
	location    string
	datePosted  string
	description string
}

// parseJobListings extracts job listings from TechCareers HTML.
func parseJobListings(html string) []parsedJob {
	matches := jobCardRe.FindAllStringSubmatch(html, -1)
	jobs := make([]parsedJob, 0, len(matches))

	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		block := m[1]

		title := extractByClass(block, "job-title")
		if title == "" {
			title = extractLink(block)
		}
		if title == "" {
			continue
		}

		href := extractHref(block)
		url := ""
		if href != "" {
			if strings.HasPrefix(href, "http") {
				url = href
			} else {
				url = "https://www.techcareers.com" + href
			}
		}

		company := extractByClass(block, "company")
		if company == "" {
			company = extractByClass(block, "employer")
		}

		loc := extractByClass(block, "location")
		if loc == "" {
			loc = extractByClass(block, "job-location")
		}

		datePosted := extractByClass(block, "date")
		if datePosted == "" {
			datePosted = extractByClass(block, "posted-date")
		}

		description := extractByClass(block, "description")
		if description == "" {
			description = extractByClass(block, "snippet")
		}

		jobs = append(jobs, parsedJob{
			title:       title,
			url:         url,
			company:     company,
			location:    loc,
			datePosted:  datePosted,
			description: description,
		})
	}

	return jobs
}

// mapJob converts a parsedJob into a model.JobPost.
func mapJob(item parsedJob, searchTerm string) (model.JobPost, error) {
	if item.title == "" {
		return model.JobPost{}, fmt.Errorf("empty title")
	}

	desc := strings.TrimSpace(item.description)

	id := idFromURL(item.url)
	if id == "" {
		id = simpleHash(item.title)
	}

	job := model.JobPost{
		ID:          "techcareers-" + id,
		Title:       item.title,
		JobURL:      item.url,
		CompanyName: item.company,
		Description: desc,
		Site:        string(model.SiteTechCareers),
		ApplyMethod: "external_url",
	}

	if item.location != "" {
		job.Location = model.Location{City: item.location}
	}

	if item.datePosted != "" {
		job.DatePosted = parseDate(item.datePosted)
	}

	return job, nil
}

// idFromURL extracts a short ID from a URL.
func idFromURL(raw string) string {
	if raw == "" {
		return ""
	}
	u := strings.TrimRight(raw, "/")
	parts := strings.Split(u, "/")
	for i := len(parts) - 1; i >= 0; i-- {
		if p := strings.TrimSpace(parts[i]); p != "" {
			return p
		}
	}
	return ""
}

// simpleHash produces a consistent hash string for a given input.
func simpleHash(s string) string {
	var h int
	for i := 0; i < len(s); i++ {
		h = (h<<5 - h) + int(s[i])
	}
	if h < 0 {
		h = -h
	}
	return fmt.Sprintf("%x", h)
}

// stripTags removes all HTML tags.
func stripTags(s string) string {
	return strings.TrimSpace(stripTagsRe.ReplaceAllString(s, " "))
}

// urlQueryParams builds query parameters for TechCareers search.
func urlQueryParams(searchTerm, location string, page int) string {
	params := make([]string, 0, 3)
	if searchTerm != "" {
		params = append(params, "q="+strings.ReplaceAll(searchTerm, " ", "+"))
	}
	if location != "" {
		params = append(params, "l="+strings.ReplaceAll(location, " ", "+"))
	}
	if page > 1 {
		params = append(params, fmt.Sprintf("p=%d", page))
	}
	return strings.Join(params, "&")
}

// parseDate parses a date string in common formats.
func parseDate(v string) *time.Time {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	formats := []string{
		"2006-01-02",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05",
		time.RFC3339,
		"Mon, 02 Jan 2006",
		"January 2, 2006",
		"Jan 2, 2006",
	}
	for _, f := range formats {
		t, err := time.Parse(f, v)
		if err == nil {
			return &t
		}
	}
	return nil
}
