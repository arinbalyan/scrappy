package iosdevjobs

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

const rssURL = "https://iosdevjobs.com/jobs.rss"

var (
	reItem  = regexp.MustCompile(`(?is)<item>(.*?)</item>`)
	reTag   = regexp.MustCompile(`(?is)<%s[^>]*>(.*?)</%s>`)
	reCDATA = regexp.MustCompile(`(?is)^\s*<!\[CDATA\[(.*?)\]\]>\s*$`)
	reTitle = regexp.MustCompile(`(?is)^\s*([^:]+):\s*(.+?)\s*$`) // if title format isn't "Title @ Company"
	reHTML  = regexp.MustCompile(`(?is)<[^>]+>`)
)

type Scraper struct {
	client  *http.Client
	rssURL  string
}

func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Retries: 2, Timeout: 15 * time.Second})
	}
	return &Scraper{client: client, rssURL: rssURL}
}

func NewWithURLs(client *http.Client, endpoint string) *Scraper {
	s := New(client)
	if strings.TrimSpace(endpoint) != "" {
		s.rssURL = strings.TrimSpace(endpoint)
	}
	return s
}

func (s *Scraper) SiteName() model.Site { return model.SiteIOSDevJobs }

func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, s.rssURL, nil)
	req.Header.Set("Accept", "application/rss+xml, application/xml, text/xml")
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("iosdevjobs request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("iosdevjobs status %d", resp.StatusCode)
	}

	body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("iosdevjobs read: %w", err)
	}

	items := parseRSSItems(string(body))
	limit := input.ResultsWanted
	if limit <= 0 || limit > len(items) {
		limit = len(items)
	}
	term := strings.ToLower(strings.TrimSpace(input.SearchTerm))

	jobs := make([]model.JobPost, 0, limit)
	for _, m := range items {
		if len(jobs) >= limit {
			break
		}
		titleRaw := m.Title
		link := strings.TrimSpace(m.Link)
		desc := m.Description
		pubDate := m.PubDate

		if titleRaw == "" || link == "" {
			continue
		}
		if term != "" {
			hay := strings.ToLower(titleRaw + " " + desc)
			if !strings.Contains(hay, term) {
				continue
			}
		}

		company := ""
		title := titleRaw
		// iOSDevJobs uses "Title @ Company" format
		if idx := strings.LastIndex(titleRaw, " @ "); idx > 0 {
			company = strings.TrimSpace(titleRaw[idx+3:])
			title = strings.TrimSpace(titleRaw[:idx])
		}

		posted := parsePubDate(pubDate)
		id := "iosdj-" + idFromURL(link)

		job := model.JobPost{
			ID:          id,
			Title:       title,
			CompanyName: company,
			JobURL:      link,
			Description: htmlToText(desc),
			DatePosted:  posted,
			IsRemote:    true, // iOSDevJobs is remote-only
		}
		jobs = append(jobs, job)
	}

	if !util.HasMeaningfulJobs(jobs) {
		return nil, fmt.Errorf("iosdevjobs no parseable jobs")
	}
	return jobs, nil
}

type rssItem struct {
	Title       string
	Link        string
	GUID        string
	Description string
	PubDate     string
}

func parseRSSItems(xml string) []rssItem {
	items := regexp.MustCompile(`(?is)<item>(.*?)</item>`).FindAllStringSubmatch(xml, -1)
	out := make([]rssItem, 0, len(items))
	for _, m := range items {
		if len(m) < 2 {
			continue
		}
		chunk := m[1]
		out = append(out, rssItem{
			Title:       extractTag(chunk, "title"),
			Link:        strings.TrimSpace(extractTag(chunk, "link")),
			GUID:        extractTag(chunk, "guid"),
			Description: extractTag(chunk, "description"),
			PubDate:     extractTag(chunk, "pubDate"),
		})
	}
	return out
}

func extractTag(xml, tag string) string {
	rx := regexp.MustCompile(fmt.Sprintf(`(?is)<%s[^>]*>(.*?)</%s>`, tag, tag))
	m := rx.FindStringSubmatch(xml)
	if len(m) < 2 {
		return ""
	}
	v := strings.TrimSpace(m[1])
	if cm := reCDATA.FindStringSubmatch(v); len(cm) == 2 {
		return strings.TrimSpace(cm[1])
	}
	return strings.TrimSpace(v)
}

func htmlToText(v string) string {
	if strings.TrimSpace(v) == "" {
		return ""
	}
	v = reHTML.ReplaceAllString(v, " ")
	return strings.Join(strings.Fields(strings.TrimSpace(v)), " ")
}

func parsePubDate(v string) *time.Time {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	if t, err := time.Parse(time.RFC1123Z, v); err == nil {
		return &t
	}
	if t, err := time.Parse(time.RFC1123, v); err == nil {
		return &t
	}
	return nil
}

func idFromURL(raw string) string {
	u := strings.TrimSpace(raw)
	if u == "" {
		return "unknown"
	}
	parts := strings.Split(u, "/")
	for i := len(parts) - 1; i >= 0; i-- {
		p := strings.TrimSpace(parts[i])
		if p != "" {
			return p
		}
	}
	return "unknown"
}
