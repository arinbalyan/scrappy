package germantechjobs

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/util"
)

const feedURL = "https://germantechjobs.de/rss"

var (
	itemRe    = regexp.MustCompile(`(?is)<item>(.*?)</item>`)
	cdataRe   = regexp.MustCompile(`(?is)^\s*<!\[CDATA\[(.*?)\]\]>\s*$`)
	salaryRe  = regexp.MustCompile(`\[([\d.,]+)\s*-\s*([\d.,]+)\s*EUR\]`)
	companyRe = regexp.MustCompile(`@\s*(.+?)\s*\[`)
)

// Scraper fetches jobs from the GermanTechJobs RSS feed.
type Scraper struct {
	client  *http.Client
	feedURL string
}

// New creates a new GermanTechJobs scraper. If client is nil a default one is used.
func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Timeout: 20 * time.Second})
	}
	return &Scraper{client: client, feedURL: feedURL}
}

// NewWithFeedURL creates a new scraper with a custom endpoint (used in tests).
func NewWithFeedURL(client *http.Client, endpoint string) *Scraper {
	s := New(client)
	if strings.TrimSpace(endpoint) != "" {
		s.feedURL = endpoint
	}
	return s
}

// SiteName returns the site identifier.
func (s *Scraper) SiteName() model.Site { return model.SiteGermanTechJobs }

// Scrape fetches jobs from the GermanTechJobs RSS feed.
func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{
		"site":           s.SiteName(),
		"results_wanted": input.ResultsWanted,
		"search_term":    input.SearchTerm,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.feedURL, nil)
	if err != nil {
		return nil, fmt.Errorf("germantechjobs: build request: %w", err)
	}
	req.Header.Set("Accept", "application/rss+xml, application/xml, text/xml")
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("germantechjobs: request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("germantechjobs: status %d", resp.StatusCode)
	}

	body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("germantechjobs: read: %w", err)
	}

	items := parseRSSItems(string(body))
	util.Debug("germantechjobs: parsed items", map[string]any{"count": len(items)})

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

		// Client-side search term filtering on title, description, and category
		if term != "" {
			hay := strings.ToLower(title + " " + item.description + " " + item.category)
			if !strings.Contains(hay, term) {
				continue
			}
		}

		job := model.JobPost{
			ID:          "germantechjobs-" + extractID(item.guid, link),
			Title:       title,
			JobURL:      link,
			Description: strings.TrimSpace(item.description),
			Site:        string(s.SiteName()),
			Location:    model.Location{Country: "Germany"},
		}

		// Parse salary from title: "Job @ Company [min - max EUR]"
		if sm := salaryRe.FindStringSubmatch(title); len(sm) == 3 {
			minStr := strings.ReplaceAll(sm[1], ".", "")
			minStr = strings.ReplaceAll(minStr, ",", "")
			maxStr := strings.ReplaceAll(sm[2], ".", "")
			maxStr = strings.ReplaceAll(maxStr, ",", "")
			minVal, minErr := strconv.ParseFloat(minStr, 64)
			maxVal, maxErr := strconv.ParseFloat(maxStr, 64)
			if minErr == nil && maxErr == nil {
				job.Compensation = &model.Compensation{
					Interval:  model.IntervalYearly,
					MinAmount: &minVal,
					MaxAmount: &maxVal,
					Currency:  "EUR",
				}
			}
		}

		// Parse company from title: "Job @ Company [salary]"
		if cm := companyRe.FindStringSubmatch(title); len(cm) == 2 {
			job.CompanyName = strings.TrimSpace(cm[1])
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
		return nil, fmt.Errorf("germantechjobs: no parseable jobs")
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
	category    string
}

// parseRSSItems extracts RSS <item> blocks from the XML body using regex.
func parseRSSItems(xml string) []rssItem {
	blocks := itemRe.FindAllStringSubmatch(xml, -1)
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

// extractTag extracts the text content of an XML tag. It handles both
// CDATA-wrapped and plain content.
func extractTag(xml, tag string) string {
	// Try CDATA first
	rx := regexp.MustCompile(fmt.Sprintf(`(?is)<%s[^>]*>\s*<!\[CDATA\[([\s\S]*?)\]\]>\s*</%s>`, tag, tag))
	if m := rx.FindStringSubmatch(xml); len(m) == 2 {
		return strings.TrimSpace(m[1])
	}
	// Try plain content
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

// extractID extracts a short ID from a URL by using the last path segment.
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
	return "unknown"
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
