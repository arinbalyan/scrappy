package undpjobs

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
	rssURL      = "https://jobs.undp.org/rss_feeds/rss.xml"
	defaultWanted = 25
)

var (
	itemBlockRe = regexp.MustCompile(`(?is)<item>(.*?)</item>`)
	stripHTMLRe = regexp.MustCompile(`(?is)<[^>]+>`)
)

// rssItem holds extracted fields from an RSS <item> block.
type rssItem struct {
	title       string
	link        string
	description string
	dutyStation string
	closingDate string
	org         string
	dcDate      string
}

// Scraper fetches jobs from the UNDP Jobs RSS feed.
type Scraper struct {
	client *http.Client
	rssURL string
}

// New creates a new UNDP Jobs scraper.
func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Timeout: 20 * time.Second})
	}
	return &Scraper{client: client, rssURL: rssURL}
}

// NewWithRSSURL creates a new scraper with a custom endpoint (used in tests).
func NewWithRSSURL(client *http.Client, endpoint string) *Scraper {
	s := New(client)
	if strings.TrimSpace(endpoint) != "" {
		s.rssURL = endpoint
	}
	return s
}

// SiteName returns the site identifier.
func (s *Scraper) SiteName() model.Site { return model.SiteUNDPJobs }

// Scrape fetches jobs from the UNDP Jobs RSS feed.
func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{
		"site":           s.SiteName(),
		"results_wanted": input.ResultsWanted,
		"search_term":    input.SearchTerm,
	})

	wanted := input.ResultsWanted
	if wanted <= 0 {
		wanted = defaultWanted
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.rssURL, nil)
	if err != nil {
		return nil, fmt.Errorf("undpjobs: build request: %w", err)
	}
	req.Header.Set("Accept", "application/xml, text/xml, application/rss+xml")
	req.Header.Set("User-Agent", "scrappy/0.1.0 (job-aggregator)")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("undpjobs: request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("undpjobs: status %d", resp.StatusCode)
	}

	body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("undpjobs: read: %w", err)
	}

	items := parseRSSItems(string(body))
	util.Debug("undpjobs: parsed items", map[string]any{"count": len(items)})

	if len(items) == 0 {
		return nil, fmt.Errorf("undpjobs: no jobs in RSS feed")
	}

	term := strings.ToLower(strings.TrimSpace(input.SearchTerm))
	limit := wanted
	if limit > len(items) {
		limit = len(items)
	}

	out := make([]model.JobPost, 0, limit)
	for _, item := range items {
		if len(out) >= limit {
			break
		}

		// Client-side search filter
		if term != "" && !matchesSearch(item, term) {
			continue
		}

		job, err := mapJob(item)
		if err != nil {
			continue
		}
		out = append(out, job)
	}

	util.Debug("scraper_done", map[string]any{
		"site": s.SiteName(),
		"jobs": len(out),
	})

	if !util.HasMeaningfulJobs(out) {
		return nil, fmt.Errorf("undpjobs: no parseable jobs")
	}
	return out, nil
}

// parseRSSItems extracts RSS <item> blocks from XML.
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
			description: extractTag(chunk, "description"),
			dutyStation: extractTag(chunk, "undpjobs:duty_station"),
			closingDate: extractTag(chunk, "undpjobs:closing_date"),
			org:         extractTag(chunk, "undpjobs:organization"),
			dcDate:      extractTag(chunk, "dc:date"),
		})
	}
	return items
}

// extractTag extracts text content from an XML tag, handling CDATA.
func extractTag(xml, tag string) string {
	// CDATA version
	re := regexp.MustCompile(fmt.Sprintf(`(?is)<%s[^>]*>\s*<!\[CDATA\[([\s\S]*?)\]\]>\s*</%s>`, regexp.QuoteMeta(tag), regexp.QuoteMeta(tag)))
	if m := re.FindStringSubmatch(xml); len(m) == 2 {
		return strings.TrimSpace(m[1])
	}
	// Plain version
	re = regexp.MustCompile(fmt.Sprintf(`(?is)<%s[^>]*>([\s\S]*?)</%s>`, regexp.QuoteMeta(tag), regexp.QuoteMeta(tag)))
	if m := re.FindStringSubmatch(xml); len(m) == 2 {
		return strings.TrimSpace(m[1])
	}
	return ""
}

// matchesSearch checks if an item matches the search term.
func matchesSearch(item rssItem, term string) bool {
	return strings.Contains(strings.ToLower(item.title), term) ||
		strings.Contains(strings.ToLower(item.description), term) ||
		strings.Contains(strings.ToLower(item.org), term)
}

// mapJob converts an rssItem to a model.JobPost.
func mapJob(item rssItem) (model.JobPost, error) {
	if strings.TrimSpace(item.title) == "" || strings.TrimSpace(item.link) == "" {
		return model.JobPost{}, fmt.Errorf("empty title or link")
	}

	desc := stripHTML(item.description)
	id := idFromURL(item.link)
	if id == "" {
		id = simpleHash(item.title)
	}

	org := strings.TrimSpace(item.org)
	if org == "" {
		org = "UNDP"
	}

	job := model.JobPost{
		ID:          "undpjobs-" + id,
		Title:       strings.TrimSpace(item.title),
		CompanyName: org,
		JobURL:      strings.TrimSpace(item.link),
		Description: desc,
		Site:        string(model.SiteUNDPJobs),
		ApplyMethod: "external_url",
	}

	if item.dutyStation != "" {
		job.Location = model.Location{City: strings.TrimSpace(item.dutyStation)}
	}

	if item.dcDate != "" {
		job.DatePosted = parseDate(strings.TrimSpace(item.dcDate))
	}

	return job, nil
}

// stripHTML removes HTML tags from a string.
func stripHTML(s string) string {
	return strings.TrimSpace(stripHTMLRe.ReplaceAllString(s, " "))
}

// idFromURL extracts a short ID from the last URL path segment.
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

// simpleHash produces a hash string for fallback IDs.
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

// parseDate parses a date string in common formats.
func parseDate(v string) *time.Time {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05",
		"2006-01-02",
		"Mon, 02 Jan 2006 15:04:05 -0700",
		"Mon, 02 Jan 2006 15:04:05 MST",
		"2006-01-02T15:04:05.000Z",
	}
	for _, f := range formats {
		t, err := time.Parse(f, v)
		if err == nil {
			return &t
		}
	}
	return nil
}
