// NICHE: WordPress-specific jobs. Returns 0 for general tech searches.
package wordpressjobs

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
	rssURL = "https://jobs.wordpress.net/feed/"
)

var (
	itemBlockRe = regexp.MustCompile(`(?is)<item>(.*?)</item>`)
	cdataRe     = regexp.MustCompile(`(?is)^\s*<!\[CDATA\[(.*?)\]\]>\s*$`)
)

// Scraper fetches jobs from the WordPress Jobs RSS feed.
type Scraper struct {
	client *http.Client
	rssURL string
}

// New creates a new WordPress Jobs scraper.
func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{
			Timeout: 20 * time.Second,
			Retries: 2,
		})
	}
	return &Scraper{client: client, rssURL: rssURL}
}

// NewWithRSSURL creates a scraper with a custom RSS URL (used in tests).
func NewWithRSSURL(client *http.Client, rssURL string) *Scraper {
	s := New(client)
	if strings.TrimSpace(rssURL) != "" {
		s.rssURL = strings.TrimSpace(rssURL)
	}
	return s
}

// SiteName returns the site identifier.
func (s *Scraper) SiteName() model.Site { return model.SiteWordPressJobs }

// Scrape fetches jobs from WordPress Jobs RSS feed.
func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{
		"site":           s.SiteName(),
		"results_wanted": input.ResultsWanted,
		"search_term":    input.SearchTerm,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.rssURL, nil)
	if err != nil {
		return nil, fmt.Errorf("wordpressjobs: build request: %w", err)
	}
	req.Header.Set("Accept", "application/rss+xml, application/xml, text/xml")
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; scrappy/1.0)")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("wordpressjobs: request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("wordpressjobs: status %d", resp.StatusCode)
	}

	body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("wordpressjobs: read: %w", err)
	}

	items := parseRSSItems(string(body))
	util.Debug("wordpressjobs: parsed items", map[string]any{"count": len(items)})

	terms := parseSearchTerms(input.SearchTerm)
	wanted := input.ResultsWanted
	if wanted <= 0 {
		wanted = 25
	}

	out := make([]model.JobPost, 0, wanted)
	for _, item := range items {
		if len(out) >= wanted {
			break
		}

		title := strings.TrimSpace(item.title)
		link := strings.TrimSpace(item.link)
		if title == "" || link == "" {
			continue
		}

		// Client-side search term filtering
		if len(terms) > 0 {
			hay := strings.ToLower(title + " " + item.description)
			if !matchAny(hay, terms) {
				continue
			}
		}

		job := model.JobPost{
			ID:          "wpjobs-" + extractID(item.guid, item.link),
			Title:       title,
			JobURL:      link,
			Description: strings.TrimSpace(item.description),
			Site:        string(s.SiteName()),
		}

		if item.pubDate != "" {
			job.DatePosted = parseDate(item.pubDate)
		}

		out = append(out, job)
	}

	util.Debug("scraper_done", map[string]any{
		"site": s.SiteName(),
		"jobs": len(out),
	})

	if !util.HasMeaningfulJobs(out) {
		return nil, fmt.Errorf("wordpressjobs: no parseable jobs")
	}
	return out, nil
}

// --- RSS parsing ---

type rssItem struct {
	title       string
	link        string
	guid        string
	description string
	pubDate     string
	category    string
}

func parseRSSItems(xml string) []rssItem {
	blocks := itemBlockRe.FindAllStringSubmatch(xml, -1)
	items := make([]rssItem, 0, len(blocks))
	for _, m := range blocks {
		if len(m) < 2 {
			continue
		}
		chunk := m[1]
		items = append(items, rssItem{
			title:       extractTag(chunk, "title"),
			link:        extractTag(chunk, "link"),
			guid:        extractTag(chunk, "guid"),
			description: extractTag(chunk, "description"),
			pubDate:     extractTag(chunk, "pubDate"),
			category:    extractTag(chunk, "category"),
		})
	}
	return items
}

func extractTag(xml, tag string) string {
	re := regexp.MustCompile(fmt.Sprintf(`(?is)<%s[^>]*>\s*<!\[CDATA\[([\s\S]*?)\]\]>\s*</%s>`, regexp.QuoteMeta(tag), regexp.QuoteMeta(tag)))
	if m := re.FindStringSubmatch(xml); len(m) == 2 {
		return strings.TrimSpace(m[1])
	}
	re = regexp.MustCompile(fmt.Sprintf(`(?is)<%s[^>]*>([\s\S]*?)</%s>`, regexp.QuoteMeta(tag), regexp.QuoteMeta(tag)))
	if m := re.FindStringSubmatch(xml); len(m) == 2 {
		v := strings.TrimSpace(m[1])
		if cm := cdataRe.FindStringSubmatch(v); len(cm) == 2 {
			return strings.TrimSpace(cm[1])
		}
		return v
	}
	return ""
}

func extractID(guid, link string) string {
	if guid != "" {
		// Try to extract post ID from /?p=NNNN format
		if idx := strings.Index(guid, "?p="); idx >= 0 {
			id := guid[idx+3:]
			if andIdx := strings.Index(id, "&"); andIdx >= 0 {
				id = id[:andIdx]
			}
			if id != "" {
				return id
			}
		}
		if idx := strings.LastIndex(guid, "/"); idx >= 0 {
			id := guid[idx+1:]
			// Skip query string
			if qIdx := strings.Index(id, "?"); qIdx >= 0 {
				id = id[:qIdx]
			}
			if id != "" {
				return id
			}
		}
	}
	if link != "" {
		u := strings.TrimRight(link, "/")
		parts := strings.Split(u, "/")
		for i := len(parts) - 1; i >= 0; i-- {
			if p := strings.TrimSpace(parts[i]); p != "" {
				if qIdx := strings.Index(p, "?"); qIdx >= 0 {
					p = p[:qIdx]
				}
				return p
			}
		}
	}
	return "unknown"
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

func parseDate(v string) *time.Time {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	formats := []string{
		time.RFC1123Z,
		time.RFC1123,
		time.RFC822Z,
		time.RFC822,
		"Mon, 02 Jan 2006 15:04:05 -0700",
		"Mon, 02 Jan 2006 15:04:05 MST",
		time.RFC3339,
		"2006-01-02T15:04:05Z",
		"2006-01-02",
	}
	for _, f := range formats {
		t, err := time.Parse(f, v)
		if err == nil {
			return &t
		}
	}
	return nil
}
