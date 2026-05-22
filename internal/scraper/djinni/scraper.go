// Package djinni implements a job scraper for djinni.co using its RSS feed.
//
// Djinni is a Ukrainian/European job board for tech professionals. The scraper
// fetches the RSS feed at https://djinni.co/jobs/rss/, parses items with
// regex-based XML extraction (matching the TypeScript source), and maps
// results to model.JobPost. There is no pagination — the RSS feed returns
// all current listings in a single response.
package djinni

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

const rssURL = "https://djinni.co/jobs/rss/"

// defaultHeaders matches the TypeScript source constants.
var defaultHeaders = map[string]string{
	"Accept":     "application/rss+xml, application/xml, text/xml",
	"User-Agent": "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/129.0.0.0 Safari/537.36",
}

// htmlTagRe strips HTML tags from descriptions.
var htmlTagRe = regexp.MustCompile(`<[^>]*>`)

// Scraper implements the scraper.Scraper interface for Djinni.
type Scraper struct {
	client *http.Client
	rssURL string
}

// New creates a new Djinni scraper. If client is nil, a default HTTP client
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

// SiteName returns model.SiteDjinni.
func (s *Scraper) SiteName() model.Site { return model.SiteDjinni }

// rssItem holds the parsed fields of a single RSS <item> from Djinni.
type rssItem struct {
	Title       string
	Link        string
	GUID        string
	Description string
	PubDate     string
	Category    string
}

// Scrape fetches the Djinni RSS feed, parses items, filters by the optional
// search term, and maps results to model.JobPost. Rate limiting (3 req/s) is
// handled by the engine's per-site semaphore.
func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	wanted := input.ResultsWanted
	if wanted <= 0 {
		wanted = 15
	}

	body, err := s.fetchRSS(ctx)
	if err != nil {
		return nil, fmt.Errorf("djinni fetch: %w", err)
	}

	items := parseRSSItems(string(body))
	if len(items) == 0 {
		return nil, fmt.Errorf("djinni no items in rss feed")
	}

	jobs := make([]model.JobPost, 0, wanted)
	searchTerm := strings.ToLower(strings.TrimSpace(input.SearchTerm))

	for _, item := range items {
		if len(jobs) >= wanted {
			break
		}

		// Filter by search term if one is provided.
		if searchTerm != "" && !matchesSearch(item, searchTerm) {
			continue
		}

		job := mapToJobPost(item)
		if job != nil {
			jobs = append(jobs, *job)
		}
	}

	if !util.HasMeaningfulJobs(jobs) {
		return nil, fmt.Errorf("djinni no parseable jobs")
	}

	if len(jobs) > wanted {
		jobs = jobs[:wanted]
	}

	return jobs, nil
}

// fetchRSS does a single HTTP GET to the Djinni RSS endpoint and returns the
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
		return nil, fmt.Errorf("djinni status %d", resp.StatusCode)
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
			Category:    extractTag(itemContent, "category"),
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

// matchesSearch checks whether an RSS item's title, description, or category
// contains the given search term (case-insensitive).
func matchesSearch(item rssItem, term string) bool {
	term = strings.ToLower(term)
	title := strings.ToLower(item.Title)
	desc := strings.ToLower(item.Description)
	cat := strings.ToLower(item.Category)

	return strings.Contains(title, term) ||
		strings.Contains(desc, term) ||
		strings.Contains(cat, term)
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

	// Generate a stable ID from the GUID or URL.
	id := extractID(item.GUID, item.Link)

	// Detect remote positions by keyword in title/description.
	isRemote := checkRemote(item)

	return &model.JobPost{
		ID:          "dj-" + id,
		Title:       item.Title,
		CompanyName: "", // Company name is not available in the RSS feed.
		JobURL:      item.Link,
		Description: description,
		DatePosted:  datePosted,
		IsRemote:    isRemote,
	}
}

// checkRemote returns true if "remote" appears in the item's title or
// description (case-insensitive).
func checkRemote(item rssItem) bool {
	title := strings.ToLower(item.Title)
	desc := strings.ToLower(item.Description)
	return strings.Contains(title, "remote") || strings.Contains(desc, "remote")
}

// extractID returns a stable identifier from the GUID or link.
// It prefers the last path segment of the URL for human readability,
// falling back to an FNV-1a hash if URL parsing fails.
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
func hashString(s string) string {
	h := fnv.New64a()
	_, _ = h.Write([]byte(s))
	return fmt.Sprintf("%d", h.Sum64())
}
