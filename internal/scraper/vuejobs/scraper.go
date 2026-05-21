package vuejobs

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

const feedURL = "https://vuejobs.com/feed"

var (
	vjItem      = regexp.MustCompile(`(?is)<item>(.*?)</item>`)
	vjTag       = regexp.MustCompile(`(?is)<%s[^>]*>(.*?)</%s>`)
	vjCDATA     = regexp.MustCompile(`(?is)^\s*<!\[CDATA\[(.*?)\]\]>\s*$`)
	vjStripTags = regexp.MustCompile(`(?is)<[^>]+>`)
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

func (s *Scraper) SiteName() model.Site { return model.SiteVueJobs }

func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, s.feedURL, nil)
	req.Header.Set("Accept", "application/rss+xml, application/xml, text/xml")
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("vuejobs request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("vuejobs status %d", resp.StatusCode)
	}

	body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("vuejobs read: %w", err)
	}

	items := vjItem.FindAllStringSubmatch(string(body), -1)
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
		chunk := m[1]
		title := extractVJTag(chunk, "title")
		link := extractVJTag(chunk, "link")
		desc := extractVJTag(chunk, "description")
		pubDate := extractVJTag(chunk, "pubDate")

		if title == "" || link == "" {
			continue
		}
		if term != "" {
			hay := strings.ToLower(title + " " + desc)
			if !strings.Contains(hay, term) {
				continue
			}
		}

		jobs = append(jobs, model.JobPost{
			ID:          "vuejobs-" + idFromURL(link),
			Title:       strings.TrimSpace(title),
			JobURL:      strings.TrimSpace(link),
			Description: vjHTMLToText(desc),
			DatePosted:  parseRFCDate(pubDate),
		})
	}

	if !util.HasMeaningfulJobs(jobs) {
		return nil, fmt.Errorf("vuejobs no parseable jobs")
	}
	return jobs, nil
}

func extractVJTag(xml, tag string) string {
	rx := regexp.MustCompile(fmt.Sprintf(vjTag.String(), tag, tag))
	m := rx.FindStringSubmatch(xml)
	if len(m) < 2 {
		return ""
	}
	v := strings.TrimSpace(m[1])
	if cm := vjCDATA.FindStringSubmatch(v); len(cm) == 2 {
		return strings.TrimSpace(cm[1])
	}
	return strings.TrimSpace(v)
}

func vjHTMLToText(v string) string {
	if strings.TrimSpace(v) == "" {
		return ""
	}
	v = vjStripTags.ReplaceAllString(v, " ")
	return strings.Join(strings.Fields(strings.TrimSpace(v)), " ")
}

func parseRFCDate(v string) *time.Time {
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
