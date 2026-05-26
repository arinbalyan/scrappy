package devitjobs

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

const feedURL = "https://devitjobs.com/api/jobsXML"

var (
	// DevITJobs uses <job> elements (not standard RSS <item>)
	jobBlockRe = regexp.MustCompile(`(?is)<job>(.*?)</job>`)
	cdataRe    = regexp.MustCompile(`(?is)^\s*<!\[CDATA\[(.*?)\]\]>\s*$`)
	salaryRe   = regexp.MustCompile(`[\d,]+`)
)

// Scraper fetches jobs from the DevITJobs XML feed.
type Scraper struct {
	client  *http.Client
	feedURL string
}

// New creates a new DevITJobs scraper.
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
func (s *Scraper) SiteName() model.Site { return model.SiteDevITJobs }

// Scrape fetches jobs from the DevITJobs XML feed.
func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{
		"site":           s.SiteName(),
		"results_wanted": input.ResultsWanted,
		"search_term":    input.SearchTerm,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.feedURL, nil)
	if err != nil {
		return nil, fmt.Errorf("devitjobs: build request: %w", err)
	}
	req.Header.Set("Accept", "application/xml, text/xml")
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("devitjobs: request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("devitjobs: status %d", resp.StatusCode)
	}

	body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("devitjobs: read: %w", err)
	}

	items := parseItems(string(body))
	util.Debug("devitjobs: parsed items", map[string]any{"count": len(items)})

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

		// Client-side search term filtering
		if term != "" {
			hay := strings.ToLower(title + " " + item.description + " " + item.company + " " + item.category)
			if !strings.Contains(hay, term) {
				continue
			}
		}

		compensation := parseSalary(item.salary)

		location := model.Location{City: strings.TrimSpace(item.location)}

		isRemote := strings.Contains(strings.ToLower(item.typeStr), "remote") ||
			strings.Contains(strings.ToLower(item.location), "remote")

		job := model.JobPost{
			ID:           "devitjobs-" + extractID(item.link),
			Title:        title,
			CompanyName:  strings.TrimSpace(item.company),
			JobURL:       link,
			Location:     location,
			Description:  strings.TrimSpace(item.description),
			Compensation: compensation,
			IsRemote:     isRemote,
			Site:         string(s.SiteName()),
		}

		// Parse date
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
		return nil, fmt.Errorf("devitjobs: no parseable jobs")
	}
	return out, nil
}

// xmlItem holds the extracted fields from a <job> block.
type xmlItem struct {
	title       string
	link        string
	description string
	company     string
	location    string
	salary      string
	pubDate     string
	category    string
	typeStr     string
}

// parseItems extracts <job> blocks from the XML body using regex.
func parseItems(xml string) []xmlItem {
	blocks := jobBlockRe.FindAllStringSubmatch(xml, -1)
	items := make([]xmlItem, 0, len(blocks))
	for _, m := range blocks {
		if len(m) < 2 {
			continue
		}
		chunk := m[1]
		items = append(items, xmlItem{
			title:       extractTag(chunk, "title"),
			link:        extractTag(chunk, "link"),
			description: extractTag(chunk, "description"),
			company:     extractTag(chunk, "company"),
			location:    extractTag(chunk, "location"),
			salary:      extractTag(chunk, "salary"),
			pubDate:     extractTag(chunk, "pubDate"),
			category:    extractTag(chunk, "category"),
			typeStr:     extractTag(chunk, "type"),
		})
	}
	return items
}

// extractTag extracts the text content of an XML tag.
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
func extractID(raw string) string {
	u := strings.TrimRight(raw, "/")
	parts := strings.Split(u, "/")
	for i := len(parts) - 1; i >= 0; i-- {
		if p := strings.TrimSpace(parts[i]); p != "" {
			return p
		}
	}
	return util.HashID(raw)
}

// parseSalary parses a salary string like "$120,000 - 140,000 per year".
func parseSalary(salaryStr string) *model.Compensation {
	if salaryStr == "" {
		return nil
	}
	amounts := salaryRe.FindAllString(salaryStr, -1)
	if len(amounts) == 0 {
		return nil
	}

	values := make([]float64, 0, len(amounts))
	for _, a := range amounts {
		v := strings.ReplaceAll(a, ",", "")
		var parsed float64
		if _, err := fmt.Sscanf(v, "%f", &parsed); err == nil && parsed > 100 {
			values = append(values, parsed)
		}
	}

	if len(values) == 0 {
		return nil
	}

	isHourly := strings.Contains(strings.ToLower(salaryStr), "hour") ||
		strings.Contains(strings.ToLower(salaryStr), "hr")

	interval := model.IntervalYearly
	if isHourly {
		interval = model.IntervalHourly
	}

	currency := "USD"
	if strings.Contains(salaryStr, "€") || strings.Contains(strings.ToUpper(salaryStr), "EUR") {
		currency = "EUR"
	} else if strings.Contains(salaryStr, "£") || strings.Contains(strings.ToUpper(salaryStr), "GBP") {
		currency = "GBP"
	}

	comp := &model.Compensation{
		Interval: interval,
		Currency: currency,
	}
	comp.MinAmount = &values[0]
	if len(values) > 1 {
		comp.MaxAmount = &values[1]
	}
	return comp
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
