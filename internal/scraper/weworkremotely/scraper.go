package weworkremotely

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
	rssURL = "https://weworkremotely.com/remote-jobs.rss"
)

var (
	itemBlockRe = regexp.MustCompile(`(?is)<item>(.*?)</item>`)
	cdataRe     = regexp.MustCompile(`(?is)^\s*<!\[CDATA\[(.*?)\]\]>\s*$`)
)

// Scraper fetches jobs from the We Work Remotely RSS feed.
type Scraper struct {
	client *http.Client
	rssURL string
}

// New creates a new WeWorkRemotely scraper.
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
func (s *Scraper) SiteName() model.Site { return model.SiteWeWorkRemotely }

// Scrape fetches jobs from We Work Remotely RSS feed.
func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{
		"site":           s.SiteName(),
		"results_wanted": input.ResultsWanted,
		"search_term":    input.SearchTerm,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.rssURL, nil)
	if err != nil {
		return nil, fmt.Errorf("weworkremotely: build request: %w", err)
	}
	req.Header.Set("Accept", "application/rss+xml, application/xml, text/xml")
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; scrappy/1.0)")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("weworkremotely: request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("weworkremotely: status %d", resp.StatusCode)
	}

	body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("weworkremotely: read: %w", err)
	}

	items := parseRSSItems(string(body))
	util.Debug("weworkremotely: parsed items", map[string]any{"count": len(items)})

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

		if item.title == "" || item.link == "" {
			continue
		}

		// Client-side search term filtering
		if term != "" {
			hay := strings.ToLower(item.title + " " + item.skills)
			if !strings.Contains(hay, term) {
				continue
			}
		}

		// Parse title format "Company: Job Title"
		companyName := ""
		jobTitle := item.title
		if idx := strings.Index(item.title, ": "); idx > 0 {
			companyName = strings.TrimSpace(item.title[:idx])
			jobTitle = strings.TrimSpace(item.title[idx+2:])
		}

		// Build location
		loc := model.Location{}
		if item.country != "" {
			loc.Country = item.country
		}
		if item.region != "" {
			loc.City = item.region
		}
		if item.state != "" {
			loc.State = item.state
		}

		job := model.JobPost{
			ID:          "wwr-" + extractID(item.guid, item.link),
			Title:       jobTitle,
			CompanyName: companyName,
			JobURL:      item.link,
			Description: strings.TrimSpace(item.description),
			Site:        string(s.SiteName()),
			IsRemote:    true,
			Location:    loc,
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
		return nil, fmt.Errorf("weworkremotely: no parseable jobs")
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
	region      string
	country     string
	state       string
	skills      string
	category    string
	jobType     string
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
			region:      extractTag(chunk, "region"),
			country:     extractTag(chunk, "country"),
			state:       extractTag(chunk, "state"),
			skills:      extractTag(chunk, "skills"),
			category:    extractTag(chunk, "category"),
			jobType:     extractTag(chunk, "type"),
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
		if idx := strings.LastIndex(guid, "/"); idx >= 0 {
			id := guid[idx+1:]
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
