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
	rssURL       = "https://jobs.undp.org/rss_feeds/rss.xml"
	defaultWanted = 25
)

var (
	itemBlockRe = regexp.MustCompile(`(?is)<item>(.*?)</item>`)
	cdataRe     = regexp.MustCompile(`(?is)^\s*<!\[CDATA\[(.*?)\]\]>\s*$`)
)

// Scraper fetches jobs from the UNDP RSS feed.
type Scraper struct {
	client  *http.Client
	rssURL  string
}

// New creates a new UNDP Jobs scraper.
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
func (s *Scraper) SiteName() model.Site { return model.SiteUNDPJobs }

// Scrape fetches jobs from the UNDP RSS feed.
func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{
		"site":           s.SiteName(),
		"results_wanted": input.ResultsWanted,
		"search_term":    input.SearchTerm,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.rssURL, nil)
	if err != nil {
		return nil, fmt.Errorf("undpjobs: build request: %w", err)
	}
	req.Header.Set("Accept", "application/rss+xml, application/xml, text/xml")
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; scrappy/1.0)")

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

	term := strings.ToLower(strings.TrimSpace(input.SearchTerm))
	wanted := input.ResultsWanted
	if wanted <= 0 {
		wanted = defaultWanted
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

		// Client-side search term filtering
		if term != "" {
			hay := strings.ToLower(title + " " + item.description + " " + item.organization)
			if !strings.Contains(hay, term) {
				continue
			}
		}

		org := item.organization
		if org == "" {
			org = "UNDP"
		}

		job := model.JobPost{
			ID:          "undpjobs-" + extractID(link),
			Title:       title,
			CompanyName: org,
			JobURL:      link,
			Description: strings.TrimSpace(item.description),
			Site:        string(s.SiteName()),
		}

		if item.dutyStation != "" {
			job.Location = model.Location{City: item.dutyStation}
		}

		if item.dcDate != "" {
			job.DatePosted = parseDate(item.dcDate)
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

// --- RSS parsing ---

type rssItem struct {
	title       string
	link        string
	description string
	dutyStation string
	closingDate string
	organization string
	dcDate       string
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
			description: extractTag(chunk, "description"),
			dutyStation: extractTag(chunk, "undpjobs:duty_station"),
			closingDate: extractTag(chunk, "undpjobs:closing_date"),
			organization: extractTag(chunk, "undpjobs:organization"),
			dcDate:      extractTag(chunk, "dc:date"),
		})
	}
	return items
}

func extractTag(xml, tag string) string {
	// Try CDATA pattern first
	re := regexp.MustCompile(fmt.Sprintf(`(?is)<%s[^>]*>\s*<!\[CDATA\[([\s\S]*?)\]\]>\s*</%s>`, regexp.QuoteMeta(tag), regexp.QuoteMeta(tag)))
	if m := re.FindStringSubmatch(xml); len(m) == 2 {
		return strings.TrimSpace(m[1])
	}
	// Try plain content
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

func extractID(raw string) string {
	if raw == "" {
		return ""
	}
	// Try to extract cur_job_id query parameter
	if idx := strings.Index(raw, "cur_job_id="); idx >= 0 {
		rest := raw[idx+11:]
		if andIdx := strings.Index(rest, "&"); andIdx >= 0 {
			return rest[:andIdx]
		}
		return rest
	}
	// Fallback: use last path segment
	u := strings.TrimRight(raw, "/")
	parts := strings.Split(u, "/")
	for i := len(parts) - 1; i >= 0; i-- {
		if p := strings.TrimSpace(parts[i]); p != "" {
			// Remove any query string
			if qIdx := strings.Index(p, "?"); qIdx >= 0 {
				p = p[:qIdx]
			}
			return p
		}
	}
	return "unknown"
}

func parseDate(v string) *time.Time {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	formats := []string{
		time.RFC3339,
		time.RFC1123Z,
		time.RFC1123,
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05",
		"2006-01-02",
		"Mon, 02 Jan 2006 15:04:05 -0700",
	}
	for _, f := range formats {
		t, err := time.Parse(f, v)
		if err == nil {
			return &t
		}
	}
	return nil
}
