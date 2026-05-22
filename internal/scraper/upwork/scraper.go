// Package upwork scrapes job listings from Upwork's public search page.
//
// The scraper fetches https://www.upwork.com/search/jobs/?q=<term>&sort=recency
// and extracts job data from JSON-LD structured data embedded in the page.
// If JSON-LD is unavailable, it falls back to regex-based HTML parsing.
//
// Rate limit: 3 requests/second (configured via 334ms ticker between pages).
package upwork

import (
	"bytes"
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

const (
	searchURL     = "https://www.upwork.com/search/jobs/"
	defaultWanted = 20
	rateLimitDur  = 334 * time.Millisecond // ~3 req/s
	maxPages      = 10
)

var (
	// Match Upwork job ID (ciphertext) in HTML href attributes.
	// Upwork job URLs follow the pattern: /jobs/~ciphertext
	jobURLRe = regexp.MustCompile(`/jobs/~([a-zA-Z0-9_~-]+)`)

	// Match JSON-LD script blocks in the page head/body.
	// Uses non-greedy [^>]*? for attributes so type= is not consumed,
	// and [\s\S]*? to capture content across newlines (Go regex . does not match \n).
	jsonLDRe = regexp.MustCompile(`<script[^>]*?type="application/ld\+json"[^>]*>([\s\S]*?)</script>`)

	// Fallback HTML: match job tile heading links.
	titleRe = regexp.MustCompile(`<h[234][^>]*class="[^"]*job-tile-title[^"]*"[^>]*>\s*<a[^>]*>\s*([^<]+?)\s*</a>\s*</h[234]>`)
)

// ---- JSON-LD types for structured data extraction ----

type jsonLDItemList struct {
	ItemListElement []jsonLDListItem `json:"itemListElement"`
}

type jsonLDListItem struct {
	Item jsonLDJobPosting `json:"item"`
}

type jsonLDJobPosting struct {
	Title       string              `json:"title"`
	URL         string              `json:"url"`
	Description string              `json:"description"`
	DatePosted  string              `json:"datePosted"`
	Org         *jsonLDOrganization `json:"hiringOrganization"`
}

type jsonLDOrganization struct {
	Name string `json:"name"`
}

// Scraper implements the scraper.Scraper interface for Upwork.
type Scraper struct {
	client    *http.Client
	searchURL string
}

// New creates a new Upwork scraper. If client is nil, a default client with
// retries and timeout is created via util.NewHTTPClient.
func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{
			Retries: 3,
			Timeout: 25 * time.Second,
		})
	}
	return &Scraper{
		client:    client,
		searchURL: searchURL,
	}
}

// NewWithURLs creates a scraper with a custom endpoint URL.
// Used by contract tests to inject a test server URL.
func NewWithURLs(client *http.Client, endpoint string) *Scraper {
	s := New(client)
	if v := strings.TrimSpace(endpoint); v != "" {
		s.searchURL = strings.TrimRight(v, "/")
	}
	return s
}

// SiteName returns the model.Site identifier for Upwork.
func (s *Scraper) SiteName() model.Site { return model.SiteUpwork }

// Scrape fetches job listings from Upwork's public search page, parsing
// structured data from JSON-LD or falling back to HTML regex extraction.
// Respects context cancellation and rate-limits to ~3 req/s between pages.
func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	wanted := input.ResultsWanted
	if wanted <= 0 {
		wanted = defaultWanted
	}
	searchTerm := strings.TrimSpace(input.SearchTerm)
	if searchTerm == "" {
		return nil, fmt.Errorf("upwork: search term required")
	}

	query := url.Values{}
	query.Set("q", searchTerm)
	query.Set("sort", "recency")
	if input.IsRemote {
		query.Set("work_scope", "remote")
	}

	allJobs := make([]model.JobPost, 0, wanted)

	for page := 1; page <= maxPages && len(allJobs) < wanted; page++ {
		select {
		case <-ctx.Done():
			return allJobs, ctx.Err()
		default:
		}

		body, err := s.fetchPage(ctx, query, page)
		if err != nil {
			return nil, fmt.Errorf("upwork page %d: %w", page, err)
		}

		jobs := s.parseJobs(body)
		if len(jobs) == 0 {
			break // no more results
		}

		allJobs = append(allJobs, jobs...)

		// Rate-limit between pages: ~3 req/s
		select {
		case <-time.After(rateLimitDur):
		case <-ctx.Done():
			return allJobs, ctx.Err()
		}
	}

	if !util.HasMeaningfulJobs(allJobs) {
		return nil, fmt.Errorf("upwork: no parseable jobs")
	}
	if len(allJobs) > wanted {
		allJobs = allJobs[:wanted]
	}
	return allJobs, nil
}

// fetchPage sends an HTTP GET to the Upwork search page with the given
// query parameters and page number. Returns the raw response body.
func (s *Scraper) fetchPage(ctx context.Context, query url.Values, page int) ([]byte, error) {
	u, err := url.Parse(s.searchURL)
	if err != nil {
		return nil, fmt.Errorf("upwork parse url: %w", err)
	}

	q := u.Query()
	for k, vs := range query {
		for _, v := range vs {
			q.Add(k, v)
		}
	}
	if page > 1 {
		q.Set("page", fmt.Sprintf("%d", page))
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("upwork request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("upwork status %d", resp.StatusCode)
	}

	body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("upwork read: %w", err)
	}
	return body, nil
}

// parseJobs tries to extract jobs from JSON-LD first, falling back to
// regex-based HTML parsing if no structured data is found.
func (s *Scraper) parseJobs(body []byte) []model.JobPost {
	if jobs := parseJSONLD(body); len(jobs) > 0 {
		return jobs
	}
	return parseHTML(body)
}

// parseJSONLD extracts job listings from Schema.org JSON-LD structured data
// embedded in the page (<script type="application/ld+json">).
// Handles both ItemList wrappers and direct JobPosting schemas.
func parseJSONLD(body []byte) []model.JobPost {
	matches := jsonLDRe.FindAllSubmatch(body, -1)
	if len(matches) == 0 {
		return nil
	}

	var jobs []model.JobPost
	for _, m := range matches {
		data := bytes.TrimSpace(m[1])
		if len(data) == 0 {
			continue
		}

		// Try ItemList wrapper (most common on Upwork search pages)
		var list jsonLDItemList
		if err := json.Unmarshal(data, &list); err == nil && len(list.ItemListElement) > 0 {
			for _, li := range list.ItemListElement {
				jp := li.Item
				title := strings.TrimSpace(jp.Title)
				urlStr := strings.TrimSpace(jp.URL)
				if title == "" || urlStr == "" {
					continue
				}
				name := ""
				if jp.Org != nil {
					name = strings.TrimSpace(jp.Org.Name)
				}
				jobs = append(jobs, model.JobPost{
					ID:          "up-" + hashID(urlStr),
					Title:       title,
					CompanyName: name,
					JobURL:      urlStr,
					Description: strings.TrimSpace(jp.Description),
				})
			}
			if len(jobs) > 0 {
				return jobs
			}
		}

		// Try direct JobPosting schema (single job pages)
		var posting jsonLDJobPosting
		if err := json.Unmarshal(data, &posting); err == nil && posting.Title != "" {
			urlStr := strings.TrimSpace(posting.URL)
			if urlStr == "" {
				continue
			}
			name := ""
			if posting.Org != nil {
				name = strings.TrimSpace(posting.Org.Name)
			}
			jobs = append(jobs, model.JobPost{
				ID:          "up-" + hashID(urlStr),
				Title:       strings.TrimSpace(posting.Title),
				CompanyName: name,
				JobURL:      urlStr,
				Description: strings.TrimSpace(posting.Description),
			})
			if len(jobs) > 0 {
				return jobs
			}
		}
	}
	return nil
}

// parseHTML extracts job listings from raw HTML using regex patterns.
// This is a fallback when no JSON-LD structured data is found.
func parseHTML(body []byte) []model.JobPost {
	html := string(body)

	// Extract job ciphertext IDs from URLs
	urlMatches := jobURLRe.FindAllStringSubmatch(html, -1)
	if len(urlMatches) == 0 {
		return nil
	}

	// Extract titles from heading elements
	titleMatches := titleRe.FindAllStringSubmatch(html, -1)

	seen := make(map[string]bool, len(urlMatches))
	var jobs []model.JobPost

	for i, m := range urlMatches {
		ciphertext := strings.TrimSpace(m[1])
		if ciphertext == "" || seen[ciphertext] {
			continue
		}
		seen[ciphertext] = true

		jobURL := "https://www.upwork.com/jobs/~" + ciphertext
		title := ""
		if i < len(titleMatches) && len(titleMatches[i]) >= 2 {
			title = strings.TrimSpace(titleMatches[i][1])
		}
		if title == "" {
			title = "Upwork Job"
		}

		jobs = append(jobs, model.JobPost{
			ID:     "up-" + hashID(jobURL),
			Title:  title,
			JobURL: jobURL,
		})
	}

	if len(jobs) > 30 {
		jobs = jobs[:30]
	}
	return jobs
}

// hashID generates a deterministic short ID from a string using FNV-1a 64-bit.
func hashID(s string) string {
	h := fnv.New64a()
	h.Write([]byte(s)) //nolint:errcheck // hash.Write never fails
	return fmt.Sprintf("%d", h.Sum64())
}
