// Package eurojobs implements a job scraper for eurojobs.com using its RSS feed.
//
// EuroJobs is a European job board covering Spain, Greece, Italy, Austria, and
// other European markets. The scraper fetches the RSS feed at
// https://www.eurojobs.com/rss, parses items with regex-based XML extraction
// (matching the TypeScript source), and maps results to model.JobPost. There
// is no pagination — the RSS feed returns all current listings in a single
// response. Rate limiting is handled by the engine (3 req/s).
package eurojobs

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

const rssURL = "https://www.eurojobs.com/rss"

// defaultHeaders matches the TypeScript source constants.
var defaultHeaders = map[string]string{
	"Accept":     "application/rss+xml, application/xml, text/xml",
	"User-Agent": "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/129.0.0.0 Safari/537.36",
}

// htmlTagRe strips HTML tags from descriptions.
var htmlTagRe = regexp.MustCompile(`<[^>]*>`)

// Scraper implements the scraper.Scraper interface for EuroJobs.
type Scraper struct {
	client *http.Client
	rssURL string
}

// New creates a new EuroJobs scraper. If client is nil, a default HTTP client
// with 3 retries and a 25-second timeout is created via util.NewHTTPClient.
func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Retries: 3, Timeout: 25 * time.Second})
	}
	return &Scraper{client: client, rssURL: rssURL}
}

// NewWithURLs creates a scraper with a custom RSS endpoint override.
// Used for testing with httptest.NewServer.
func NewWithURLs(client *http.Client, endpoint string) *Scraper {
	s := New(client)
	if strings.TrimSpace(endpoint) != "" {
		s.rssURL = strings.TrimSpace(endpoint)
	}
	return s
}

// SiteName returns model.SiteEuroJobs.
func (s *Scraper) SiteName() model.Site { return model.SiteEuroJobs }

// rssItem holds the parsed fields of a single RSS <item> from EuroJobs.
type rssItem struct {
	Title       string
	Link        string
	GUID        string
	Description string
	PubDate     string
}

// Scrape fetches the EuroJobs RSS feed, parses items, filters by the optional
// search term, and maps results to model.JobPost. Rate limiting (3 req/s) is
// handled by the engine's per-site semaphore.
func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	wanted := input.ResultsWanted
	if wanted <= 0 {
		wanted = 15
	}

	body, err := s.fetchRSS(ctx)
	if err != nil {
		return nil, fmt.Errorf("eurojobs fetch: %w", err)
	}

	items := parseRSSItems(string(body))
	if len(items) == 0 {
		return nil, fmt.Errorf("eurojobs no items in rss feed")
	}

	jobs := make([]model.JobPost, 0, wanted)
	searchTerms := parseSearchTerms(input.SearchTerm)

	for _, item := range items {
		if len(jobs) >= wanted {
			break
		}

		// Filter by search term if one is provided (OR semantics).
		if len(searchTerms) > 0 && !matchesSearch(item, searchTerms) {
			continue
		}

		job := mapToJobPost(item)
		if job != nil {
			jobs = append(jobs, *job)
		}
	}

	if !util.HasMeaningfulJobs(jobs) {
		return nil, fmt.Errorf("eurojobs no parseable jobs")
	}

	if len(jobs) > wanted {
		jobs = jobs[:wanted]
	}

	return jobs, nil
}

// fetchRSS does a single HTTP GET to the EuroJobs RSS endpoint and returns the
// raw response body. Status codes outside 2xx are treated as errors.
func (s *Scraper) fetchRSS(ctx context.Context) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.rssURL, nil)
	if err != nil {
		return nil, err
	}
	for k, v := range defaultHeaders {
		req.Header.Set(k, v)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("eurojobs status %d", resp.StatusCode)
	}

	body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("read: %w", err)
	}
	return body, nil
}

// parseRSSItems splits the RSS XML on <item> tags and extracts fields from
// each block using regex-based tag extraction (mirrors the TypeScript source).
func parseRSSItems(xml string) []rssItem {
	parts := strings.Split(xml, "<item>")
	if len(parts) < 2 {
		return nil
	}

	items := make([]rssItem, 0, len(parts)-1)
	for _, block := range parts[1:] {
		itemContent := block
		if idx := strings.Index(strings.ToLower(block), "</item>"); idx >= 0 {
			itemContent = block[:idx]
		}

		item := rssItem{
			Title:       extractTag(itemContent, "title"),
			Link:        extractTag(itemContent, "link"),
			GUID:        extractTag(itemContent, "guid"),
			Description: extractTag(itemContent, "description"),
			PubDate:     extractTag(itemContent, "pubDate"),
		}

		if item.Title == "" && item.Link == "" {
			continue
		}

		items = append(items, item)
	}

	return items
}

// extractTag extracts the text content of an XML tag using regex.
// It checks for CDATA-wrapped content first, then falls back to plain text.
// Case-insensitive matching is used for tag names.
func extractTag(xml, tagName string) string {
	// Try CDATA first: <tag><![CDATA[content]]></tag>
	cdataRe := regexp.MustCompile(`(?i)<` + tagName + `[^>]*>\s*<!\[CDATA\[([\s\S]*?)\]\]>\s*</` + tagName + `>`)
	if m := cdataRe.FindStringSubmatch(xml); len(m) >= 2 {
		return strings.TrimSpace(m[1])
	}

	// Try plain content: <tag>content</tag>
	plainRe := regexp.MustCompile(`(?i)<` + tagName + `[^>]*>([\s\S]*?)</` + tagName + `>`)
	if m := plainRe.FindStringSubmatch(xml); len(m) >= 2 {
		return strings.TrimSpace(m[1])
	}

	return ""
}

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

// matchesSearch checks whether an RSS item's title or description contains any
// of the given search terms (case-insensitive, OR semantics).
func matchesSearch(item rssItem, terms []string) bool {
	title := strings.ToLower(item.Title)
	desc := strings.ToLower(item.Description)

	hay := title + " " + desc
	return matchAny(hay, terms)
}

// mapToJobPost converts a parsed rssItem to a model.JobPost.
// Returns nil if the item has no title or link.
func mapToJobPost(item rssItem) *model.JobPost {
	if item.Title == "" || item.Link == "" {
		return nil
	}

	// Strip HTML tags from the description for clean plain-text output.
	description := htmlTagRe.ReplaceAllString(item.Description, "")
	description = strings.TrimSpace(description)

	// Parse the RSS pubDate (RFC 1123 format, e.g. "Tue, 21 May 2026 10:00:00 GMT").
	var datePosted *time.Time
	if item.PubDate != "" {
		t, err := time.Parse(time.RFC1123, item.PubDate)
		if err == nil {
			datePosted = &t
		} else {
			// Fallback: try RFC1123Z.
			t2, err2 := time.Parse(time.RFC1123Z, item.PubDate)
			if err2 == nil {
				datePosted = &t2
			}
		}
	}

	// Generate a stable ID from the GUID or URL (mirrors TypeScript extractIdFromUrl).
	id := extractID(item.GUID, item.Link)

	return &model.JobPost{
		ID:          "ej-" + id,
		Title:       item.Title,
		CompanyName: "", // Company name extracted from title prefix.
		JobURL:      item.Link,
		Description: description,
		DatePosted:  datePosted,
	}
}

// extractID returns a stable identifier from the GUID or link.
// It prefers the last path segment of the URL for human readability,
// falling back to an FNV-1a hash if URL parsing fails.
// Mirrors the TypeScript extractIdFromUrl implementation.
func extractID(guid, link string) string {
	// Prefer the GUID if it looks like a short ID (no slashes).
	if guid != "" && !strings.Contains(guid, "/") {
		return guid
	}

	// Otherwise, extract the last path segment from the link.
	u, err := url.Parse(link)
	if err == nil && u.Path != "" {
		segments := strings.Split(strings.Trim(u.Path, "/"), "/")
		if len(segments) > 0 && segments[len(segments)-1] != "" {
			return segments[len(segments)-1]
		}
	}

	// Fall back to a content-hash ID.
	return hashString(link)
}

// hashString produces a stable numeric hash from a string using FNV-1a 64-bit.
// Mirrors the TypeScript hashString implementation.
func hashString(s string) string {
	h := fnv.New64a()
	_, _ = h.Write([]byte(s))
	return fmt.Sprintf("%d", h.Sum64())
}
