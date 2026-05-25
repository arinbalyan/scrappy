package realworkfromanywhere

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/util"
)

const rssURL = "https://www.realworkfromanywhere.com/rss.xml"

// realworkfromanywhere is a remote-only board — IsRemote is always true.
// This scraper fetches and parses an RSS feed using regex (no XML library),
// matching the TypeScript reference implementation.

// --- RSS item types ---

// rssItem holds parsed data from an RSS <item> element.
type rssItem struct {
	Title       string
	Link        string
	GUID        string
	Description string
	PubDate     string
	Category    string
}

// Scraper fetches jobs from the Real Work From Anywhere RSS feed.
type Scraper struct {
	client *http.Client
	rssURL string
}

// New creates a new RealWorkFromAnywhere scraper. If client is nil a default one is used.
func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Timeout: 20 * time.Second})
	}
	return &Scraper{client: client, rssURL: rssURL}
}

// NewWithAPIURL creates a new scraper with a custom RSS URL (used in tests).
func NewWithAPIURL(client *http.Client, endpoint string) *Scraper {
	s := New(client)
	if strings.TrimSpace(endpoint) != "" {
		s.rssURL = endpoint
	}
	return s
}

// SiteName returns the site identifier.
func (s *Scraper) SiteName() model.Site { return model.SiteRealWorkFromAnywhere }

// Scrape fetches jobs from the RealWorkFromAnywhere RSS feed. The feed contains
// all current job listings. Client-side filtering is applied for search terms.
func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{
		"site":           s.SiteName(),
		"results_wanted": input.ResultsWanted,
		"search_term":    input.SearchTerm,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.rssURL, nil)
	if err != nil {
		return nil, fmt.Errorf("realworkfromanywhere: build request: %w", err)
	}
	req.Header.Set("Accept", "application/rss+xml, application/xml, text/xml")
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("realworkfromanywhere: request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("realworkfromanywhere: status %d", resp.StatusCode)
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("realworkfromanywhere: read: %w", err)
	}

	xml := string(raw)
	items := parseRSSItems(xml)

	if len(items) == 0 {
		return nil, fmt.Errorf("realworkfromanywhere: no items found in RSS feed")
	}

	term := strings.ToLower(strings.TrimSpace(input.SearchTerm))
	wanted := input.ResultsWanted
	if wanted <= 0 {
		wanted = 25
	}

	out := make([]model.JobPost, 0, wanted)
	for _, item := range items {
		if len(out) >= wanted {
			break
		}

		// Client-side search filter (matching TS reference).
		if term != "" && !matchesSearch(item, term) {
			continue
		}

		job := mapItem(item)
		if job == nil {
			continue
		}
		out = append(out, *job)
	}

	util.Debug("scraper_done", map[string]any{
		"site": s.SiteName(),
		"jobs": len(out),
	})

	if !util.HasMeaningfulJobs(out) {
		return nil, fmt.Errorf("realworkfromanywhere: no parseable jobs")
	}
	return out, nil
}

// parseRSSItems splits the RSS XML by <item> tags and extracts each item using regex.
// This mirrors the TypeScript reference implementation.
func parseRSSItems(xml string) []rssItem {
	items := make([]rssItem, 0)
	itemBlocks := strings.Split(xml, "<item>")

	// Skip the first split (everything before the first <item>)
	for _, block := range itemBlocks[1:] {
		// Get the content up to </item>
		itemContent := block
		if idx := strings.Index(strings.ToLower(block), "</item>"); idx >= 0 {
			itemContent = block[:idx]
		}

		items = append(items, rssItem{
			Title:       extractTag(itemContent, "title"),
			Link:        extractTag(itemContent, "link"),
			GUID:        extractTag(itemContent, "guid"),
			Description: extractTag(itemContent, "description"),
			PubDate:     extractTag(itemContent, "pubDate"),
			Category:    extractTag(itemContent, "category"),
		})
	}

	return items
}

// extractTag extracts the text content of an XML tag using regex.
// Handles both CDATA-wrapped and plain content, matching the TS reference.
func extractTag(xml, tagName string) string {
	// Try CDATA first
	cdataPattern := fmt.Sprintf(`<%s[^>]*>\s*<!\[CDATA\[([\s\S]*?)\]\]>\s*</%s>`, tagName, tagName)
	re := regexp.MustCompile(`(?i)` + cdataPattern)
	if m := re.FindStringSubmatch(xml); len(m) >= 2 {
		return strings.TrimSpace(m[1])
	}

	// Try plain content
	plainPattern := fmt.Sprintf(`<%s[^>]*>([\s\S]*?)</%s>`, tagName, tagName)
	re = regexp.MustCompile(`(?i)` + plainPattern)
	if m := re.FindStringSubmatch(xml); len(m) >= 2 {
		return strings.TrimSpace(m[1])
	}

	return ""
}

// matchesSearch checks whether an RSS item matches the given search term
// (case-insensitive) across title and description.
func matchesSearch(item rssItem, term string) bool {
	title := strings.ToLower(item.Title)
	desc := strings.ToLower(item.Description)
	return strings.Contains(title, term) || strings.Contains(desc, term)
}

// mapItem converts a parsed RSS item to a model.JobPost.
func mapItem(item rssItem) *model.JobPost {
	title := strings.TrimSpace(item.Title)
	link := strings.TrimSpace(item.Link)
	if title == "" || link == "" {
		return nil
	}

	job := model.JobPost{
		ID:       "rwfa-" + extractIDFromURL(link, item.GUID),
		Title:    title,
		Site:     string(model.SiteRealWorkFromAnywhere),
		JobURL:   link,
		IsRemote: true, // RealWorkFromAnywhere is a remote-only board
	}

	// Description
	if desc := strings.TrimSpace(item.Description); desc != "" {
		job.Description = desc
	}

	// DatePosted from pubDate (RFC 2822 date format)
	if item.PubDate != "" {
		if t, err := time.Parse(time.RFC1123Z, item.PubDate); err == nil {
			job.DatePosted = &t
		} else if t, err := time.Parse(time.RFC1123, item.PubDate); err == nil {
			job.DatePosted = &t
		} else if t, err := time.Parse("2006-01-02", item.PubDate); err == nil {
			job.DatePosted = &t
		}
	}

	return &job
}

// extractIDFromURL extracts a short ID from a URL by using the last path segment.
// Falls back to the GUID or a simple hash of the URL, matching the TS reference.
func extractIDFromURL(link, guid string) string {
	// Try to parse the URL and get the last path segment
	if u, err := url.Parse(link); err == nil {
		segments := strings.Split(strings.Trim(u.Path, "/"), "/")
		if len(segments) > 0 {
			last := segments[len(segments)-1]
			if last != "" {
				return last
			}
		}
	}

	// Fall back to GUID if available
	if guid != "" {
		// Try to extract from GUID too
		if u, err := url.Parse(guid); err == nil {
			segments := strings.Split(strings.Trim(u.Path, "/"), "/")
			if len(segments) > 0 {
				last := segments[len(segments)-1]
				if last != "" {
					return last
				}
			}
		}
		return guid
	}

	// Simple hash fallback — matches TS reference's hashString()
	return simpleHash(link)
}

// simpleHash computes a simple string hash for fallback IDs, matching the TS reference.
func simpleHash(s string) string {
	var hash int32
	for i := 0; i < len(s); i++ {
		hash = (hash<<5 - hash + int32(s[i]))
	}
	if hash < 0 {
		hash = -hash
	}
	return fmt.Sprintf("%x", int(hash))
}
