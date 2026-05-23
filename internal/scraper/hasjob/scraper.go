package hasjob

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

const feedURL = "https://hasjob.co/feed"

var (
	hjEntry      = regexp.MustCompile(`(?is)<entry>(.*?)</entry>`)
	hjTag        = regexp.MustCompile(`(?is)<%s[^>]*>(.*?)</%s>`)
	hjCDATA      = regexp.MustCompile(`(?is)^\s*<!\[CDATA\[(.*?)\]\]>\s*$`)
	hjStripTags  = regexp.MustCompile(`(?is)<[^>]+>`)
	hjAtomAltRef = regexp.MustCompile(`(?is)<link[^>]+rel="alternate"[^>]+href="([^"]+)"[^>]*/?>`)
	hjAtomAnyRef = regexp.MustCompile(`(?is)<link[^>]+href="([^"]+)"[^>]*/?>`)
)

type Scraper struct {
	client  *http.Client
	feedURL string
}

func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Retries: 2, Timeout: 15 * time.Second})
	}
	return &Scraper{client: client, feedURL: feedURL}
}

func NewWithFeedURL(client *http.Client, endpoint string) *Scraper {
	s := New(client)
	if strings.TrimSpace(endpoint) != "" {
		s.feedURL = endpoint
	}
	return s
}

func (s *Scraper) SiteName() model.Site { return model.SiteHasJob }

func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, s.feedURL, nil)
	req.Header.Set("Accept", "application/atom+xml, application/xml, text/xml")
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("hasjob request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("hasjob status %d", resp.StatusCode)
	}

	body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("hasjob read: %w", err)
	}

	entries := hjEntry.FindAllStringSubmatch(string(body), -1)
	limit := input.ResultsWanted
	if limit <= 0 || limit > len(entries) {
		limit = len(entries)
	}
	term := strings.ToLower(strings.TrimSpace(input.SearchTerm))
	jobs := make([]model.JobPost, 0, limit)

	for _, m := range entries {
		if len(jobs) >= limit {
			break
		}
		chunk := m[1]
		title := extractHJTag(chunk, "title")
		content := extractHJTag(chunk, "content")
		location := extractHJTag(chunk, "location")
		published := extractHJTag(chunk, "published")
		link := extractHJLink(chunk)

		if strings.TrimSpace(title) == "" || strings.TrimSpace(link) == "" {
			continue
		}
		if term != "" {
			hay := strings.ToLower(title + " " + content + " " + location)
			if !strings.Contains(hay, term) {
				continue
			}
		}

		jobs = append(jobs, model.JobPost{
			ID:          "hasjob-" + idFromURL(link),
			Title:       strings.TrimSpace(title),
			JobURL:      strings.TrimSpace(link),
			Description: hjHTMLToText(content),
			Location: model.Location{
				City:    strings.TrimSpace(location),
				Country: "India",
			},
			DatePosted: parseISODate(published),
		})
	}

	if !util.HasMeaningfulJobs(jobs) {
		return nil, fmt.Errorf("hasjob no parseable jobs")
	}
	return jobs, nil
}

func extractHJTag(xml, tag string) string {
	rx := regexp.MustCompile(fmt.Sprintf(hjTag.String(), tag, tag))
	m := rx.FindStringSubmatch(xml)
	if len(m) < 2 {
		return ""
	}
	v := strings.TrimSpace(m[1])
	if cm := hjCDATA.FindStringSubmatch(v); len(cm) == 2 {
		return strings.TrimSpace(cm[1])
	}
	return strings.TrimSpace(v)
}

func extractHJLink(xml string) string {
	if m := hjAtomAltRef.FindStringSubmatch(xml); len(m) == 2 {
		return strings.TrimSpace(m[1])
	}
	if m := hjAtomAnyRef.FindStringSubmatch(xml); len(m) == 2 {
		return strings.TrimSpace(m[1])
	}
	return ""
}

func hjHTMLToText(v string) string {
	if strings.TrimSpace(v) == "" {
		return ""
	}
	v = hjStripTags.ReplaceAllString(v, " ")
	return strings.Join(strings.Fields(strings.TrimSpace(v)), " ")
}

func parseISODate(v string) *time.Time {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	if t, err := time.Parse(time.RFC3339, v); err == nil {
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
