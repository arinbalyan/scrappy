package academiccareers

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
	rssURL = "https://www.academiccareers.com/rss"
)

var (
	acItem     = regexp.MustCompile(`(?is)<item>(.*?)</item>`)
	acTag      = func(tag string) *regexp.Regexp {
		// Escape regex metacharacters in tag name (for namespaced tags like dc:creator)
		escaped := regexp.QuoteMeta(tag)
		return regexp.MustCompile(fmt.Sprintf(`(?is)<%s[^>]*>(.*?)</%s>`, escaped, escaped))
	}
	acCDATA = regexp.MustCompile(`(?is)^\s*<!\[CDATA\[(.*?)\]\]>\s*$`)
	acHTML  = regexp.MustCompile(`(?is)<[^>]+>`)
)

// rssItem represents a parsed RSS item from AcademicCareers.
type rssItem struct {
	Title       string
	Link        string
	GUID        string
	Description string
	PubDate     string
	Creator     string
}

// Scraper fetches jobs from the AcademicCareers RSS feed.
type Scraper struct {
	client  *http.Client
	feedURL string
}

// New creates a new AcademicCareers scraper.
func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Timeout: 20 * time.Second})
	}
	return &Scraper{client: client, feedURL: rssURL}
}

// NewWithFeedURL creates a new scraper with a custom feed URL (used in tests).
func NewWithFeedURL(client *http.Client, endpoint string) *Scraper {
	s := New(client)
	if strings.TrimSpace(endpoint) != "" {
		s.feedURL = endpoint
	}
	return s
}

// SiteName returns the site identifier.
func (s *Scraper) SiteName() model.Site { return model.SiteAcademicCareers }

// Scrape fetches jobs from the AcademicCareers RSS feed.
func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{
		"site":           s.SiteName(),
		"results_wanted": input.ResultsWanted,
		"search_term":    input.SearchTerm,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.feedURL, nil)
	if err != nil {
		return nil, fmt.Errorf("academiccareers: build request: %w", err)
	}
	req.Header.Set("Accept", "application/rss+xml, application/xml, text/xml")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/129 Safari/537.36")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("academiccareers: request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("academiccareers: status %d — try using --proxy with a residential proxy", resp.StatusCode)
	}

	body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("academiccareers: read: %w", err)
	}

	xmlStr := string(body)
	items := parseRSSItems(xmlStr)

	limit := input.ResultsWanted
	if limit <= 0 {
		limit = 25
	}
	if limit > len(items) {
		limit = len(items)
	}

	term := strings.ToLower(strings.TrimSpace(input.SearchTerm))
	jobs := make([]model.JobPost, 0, limit)

	for _, item := range items {
		if len(jobs) >= limit {
			break
		}
		if term != "" && !matchesSearch(item, term) {
			continue
		}

		title := strings.TrimSpace(item.Title)
		link := strings.TrimSpace(item.Link)
		if title == "" || link == "" {
			continue
		}

		// Description — strip HTML for plain text
		desc := strings.TrimSpace(item.Description)
		if desc != "" {
			desc = util.StripHTML(desc)
		}

		// DatePosted
		var datePosted *time.Time
		if item.PubDate != "" {
			datePosted = parseDate(item.PubDate)
		}

		// ID from GUID or link
		jobID := extractID(item.GUID)
		if jobID == "" {
			jobID = extractID(link)
		}
		if jobID == "" {
			jobID = util.HashID(link)
		}

		job := model.JobPost{
			ID:          "academiccareers-" + jobID,
			Title:       title,
			CompanyName: strings.TrimSpace(item.Creator),
			JobURL:      link,
			Description: desc,
			DatePosted:  datePosted,
			Site:        string(model.SiteAcademicCareers),
		}
		jobs = append(jobs, job)
	}

	util.Debug("scraper_done", map[string]any{
		"site": s.SiteName(),
		"jobs": len(jobs),
	})

	if !util.HasMeaningfulJobs(jobs) {
		return nil, fmt.Errorf("academiccareers: no parseable jobs")
	}
	return jobs, nil
}

// parseRSSItems parses RSS XML into a slice of rssItem.
func parseRSSItems(xml string) []rssItem {
	blocks := acItem.FindAllStringSubmatch(xml, -1)
	items := make([]rssItem, 0, len(blocks))
	for _, m := range blocks {
		if len(m) < 2 {
			continue
		}
		chunk := m[1]
		item := rssItem{
			Title:       extractTag(chunk, "title"),
			Link:        extractTag(chunk, "link"),
			GUID:        extractTag(chunk, "guid"),
			Description: extractTag(chunk, "description"),
			PubDate:     extractTag(chunk, "pubDate"),
			Creator:     extractTag(chunk, "dc:creator"),
		}
		items = append(items, item)
	}
	return items
}

// extractTag extracts the text content of an XML tag, handling CDATA.
func extractTag(xml, tagName string) string {
	rx := acTag(tagName)
	m := rx.FindStringSubmatch(xml)
	if len(m) < 2 {
		return ""
	}
	v := strings.TrimSpace(m[1])
	if cm := acCDATA.FindStringSubmatch(v); len(cm) == 2 {
		return strings.TrimSpace(cm[1])
	}
	return v
}

// matchesSearch checks whether an RSS item matches the search term.
func matchesSearch(item rssItem, term string) bool {
	term = strings.ToLower(strings.TrimSpace(term))
	if term == "" {
		return true
	}
	hay := strings.ToLower(item.Title + " " + item.Description + " " + item.Creator)
	return strings.Contains(hay, term)
}

// extractID extracts a short ID from a URL using the last path segment.
func extractID(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	// Try URL parsing first
	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		parts := strings.Split(raw, "/")
		for i := len(parts) - 1; i >= 0; i-- {
			p := strings.TrimSpace(parts[i])
			if p != "" {
				return p
			}
		}
	}
	// Fallback: just return the raw value hashed
	return util.HashID(raw)
}

// parseDate attempts to parse a date string in various formats.
func parseDate(v string) *time.Time {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	formats := []string{
		time.RFC1123Z,
		time.RFC1123,
		time.RFC3339,
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05",
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
