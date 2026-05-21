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

const feedURL = "https://weworkremotely.com/remote-jobs.rss"

var (
	reItem  = regexp.MustCompile(`(?is)<item>(.*?)</item>`)
	reTag   = regexp.MustCompile(`(?is)<%s[^>]*>(.*?)</%s>`)
	reCDATA = regexp.MustCompile(`(?is)^\s*<!\[CDATA\[(.*?)\]\]>\s*$`)
	reTitle = regexp.MustCompile(`(?is)^\s*([^:]+):\s*(.+?)\s*$`)
	reHTML  = regexp.MustCompile(`(?is)<[^>]+>`)
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

func (s *Scraper) SiteName() model.Site { return model.SiteWeWorkRemotely }

func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, s.feedURL, nil)
	req.Header.Set("Accept", "application/rss+xml, application/xml, text/xml")
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("weworkremotely request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("weworkremotely status %d", resp.StatusCode)
	}

	body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("weworkremotely read: %w", err)
	}

	items := reItem.FindAllStringSubmatch(string(body), -1)
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
		titleRaw := extractTag(chunk, "title")
		link := strings.TrimSpace(extractTag(chunk, "link"))
		desc := extractTag(chunk, "description")
		pubDate := extractTag(chunk, "pubDate")
		skills := extractTag(chunk, "skills")
		jobType := extractTag(chunk, "type")
		region := extractTag(chunk, "region")
		country := extractTag(chunk, "country")
		state := extractTag(chunk, "state")

		if titleRaw == "" || link == "" {
			continue
		}
		if term != "" {
			hay := strings.ToLower(titleRaw + " " + skills)
			if !strings.Contains(hay, term) {
				continue
			}
		}

		company := ""
		title := titleRaw
		if mt := reTitle.FindStringSubmatch(titleRaw); len(mt) == 3 {
			company = strings.TrimSpace(mt[1])
			title = strings.TrimSpace(mt[2])
		}

		posted := parsePubDate(pubDate)
		id := "wwr-" + idFromURL(link)
		loc := model.Location{City: strings.TrimSpace(region), State: strings.TrimSpace(state), Country: strings.TrimSpace(country)}

		jobs = append(jobs, model.JobPost{
			ID:          id,
			Title:       strings.TrimSpace(title),
			CompanyName: strings.TrimSpace(company),
			JobURL:      link,
			Description: htmlToText(desc),
			DatePosted:  posted,
			Location:    loc,
			IsRemote:    true,
			JobType:     strings.ToLower(strings.TrimSpace(jobType)),
		})
	}

	if !util.HasMeaningfulJobs(jobs) {
		return nil, fmt.Errorf("weworkremotely no parseable jobs")
	}
	return jobs, nil
}

func extractTag(xml, tag string) string {
	rx := regexp.MustCompile(fmt.Sprintf(reTag.String(), tag, tag))
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
