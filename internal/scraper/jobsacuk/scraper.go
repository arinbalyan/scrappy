package jobsacuk

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

const feedURL = "https://www.jobs.ac.uk/jobs/government-and-public-sector?format=rss"

var (
	itemBlockRe = regexp.MustCompile(`(?is)<item>(.*?)</item>`)
	cdataRe     = regexp.MustCompile(`(?is)^\s*<!\[CDATA\[(.*?)\]\]>\s*$`)
)

// Scraper fetches jobs from the Jobs.ac.uk RSS feed.
type Scraper struct {
	client  *http.Client
	feedURL string
}

// New creates a new Jobs.ac.uk scraper.
func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Retries: 2, Timeout: 15 * time.Second})
	}
	return &Scraper{client: client, feedURL: feedURL}
}

// NewWithFeedURL creates a scraper with a custom endpoint (used in tests).
func NewWithFeedURL(client *http.Client, endpoint string) *Scraper {
	s := New(client)
	if strings.TrimSpace(endpoint) != "" {
		s.feedURL = endpoint
	}
	return s
}

// SiteName returns the site identifier.
func (s *Scraper) SiteName() model.Site { return model.SiteJobsAcUK }

// Scrape fetches jobs from the Jobs.ac.uk RSS feed.
func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{
		"site":           s.SiteName(),
		"results_wanted": input.ResultsWanted,
		"search_term":    input.SearchTerm,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.feedURL, nil)
	if err != nil {
		return nil, fmt.Errorf("jobsacuk: build request: %w", err)
	}
	req.Header.Set("Accept", "application/rss+xml, application/xml, text/xml")
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("jobsacuk: request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("jobsacuk: status %d", resp.StatusCode)
	}

	body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("jobsacuk: read: %w", err)
	}

	items := parseItems(string(body))
	util.Debug("jobsacuk: parsed items", map[string]any{"count": len(items)})

	term := strings.ToLower(strings.TrimSpace(input.SearchTerm))
	wanted := input.ResultsWanted
	if wanted <= 0 {
		wanted = len(items)
	}
	if wanted > len(items) {
		wanted = len(items)
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

		// Client-side search term filtering on title + description
		if term != "" {
			hay := strings.ToLower(title + " " + item.description)
			if !strings.Contains(hay, term) {
				continue
			}
		}

		job := model.JobPost{
			ID:          "jobsacuk-" + extractID(item.guid, item.link),
			Title:       title,
			JobURL:      link,
			Description: strings.TrimSpace(item.description),
			Location:    model.Location{Country: "United Kingdom"},
			Site:        string(s.SiteName()),
		}

		// Parse pubDate
		if pd := strings.TrimSpace(item.pubDate); pd != "" {
			job.DatePosted = parseRSSDate(pd)
		}

		out = append(out, job)
	}

	util.Debug("scraper_done", map[string]any{
		"site": s.SiteName(),
		"jobs": len(out),
	})

	if !util.HasMeaningfulJobs(out) {
		return nil, fmt.Errorf("jobsacuk: no parseable jobs")
	}
	return out, nil
}

// rssItem holds the extracted fields from an <item> block.
type rssItem struct {
	title       string
	link        string
	guid        string
	description string
	pubDate     string
}

// parseItems extracts RSS <item> blocks from the XML body using regex.
func parseItems(xml string) []rssItem {
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
		})
	}
	return items
}

// extractTag extracts the text content of an XML tag, handling both CDATA and plain content.
func extractTag(xml, tag string) string {
	rx := regexp.MustCompile(fmt.Sprintf(`(?is)<%s[^>]*>\s*<!\[CDATA\[([\s\S]*?)\]\]>\s*</%s>`, tag, tag))
	if m := rx.FindStringSubmatch(xml); len(m) == 2 {
		return strings.TrimSpace(m[1])
	}
	rx = regexp.MustCompile(fmt.Sprintf(`(?is)<%s[^>]*>([\s\S]*?)</%s>`, tag, tag))
	if m := rx.FindStringSubmatch(xml); len(m) == 2 {
		v := strings.TrimSpace(m[1])
		if cm := cdataRe.FindStringSubmatch(v); len(cm) == 2 {
			return strings.TrimSpace(cm[1])
		}
		return v
	}
	return ""
}

// extractID extracts a short ID from a URL using the last path segment.
func extractID(guid, link string) string {
	raw := guid
	if raw == "" {
		raw = link
	}
	u := strings.TrimRight(raw, "/")
	parts := strings.Split(u, "/")
	for i := len(parts) - 1; i >= 0; i-- {
		if p := strings.TrimSpace(parts[i]); p != "" {
			return p
		}
	}
	return util.HashID(raw)
}

// parseRSSDate attempts to parse an RSS pubDate string.
func parseRSSDate(v string) *time.Time {
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
