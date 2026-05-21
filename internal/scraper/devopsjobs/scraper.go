package devopsjobs

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/util"
)

const feedURL = "https://devopsjobs.io/jobs.rss"

var ddStripTags = regexp.MustCompile(`(?is)<[^>]+>`)

type rssItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	GUID        string `xml:"guid"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
}

type Scraper struct {
	client  *http.Client
	feedURL string
}

func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Retries: 2, Timeout: 20 * time.Second})
	}
	return &Scraper{client: client, feedURL: feedURL}
}

func NewWithFeedURL(client *http.Client, endpoint string) *Scraper {
	s := New(client)
	if strings.TrimSpace(endpoint) != "" {
		s.feedURL = strings.TrimSpace(endpoint)
	}
	return s
}

func (s *Scraper) SiteName() model.Site { return model.SiteDevOpsJobs }

func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	wanted := input.ResultsWanted
	if wanted <= 0 {
		wanted = 25
	}
	term := strings.ToLower(strings.TrimSpace(input.SearchTerm))

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, s.feedURL, nil)
	req.Header.Set("Accept", "application/rss+xml, application/xml, text/xml")
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("devopsjobs request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("devopsjobs status %d", resp.StatusCode)
	}

	dec := xml.NewDecoder(resp.Body)
	jobs := make([]model.JobPost, 0, wanted)
	for {
		tok, err := dec.Token()
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			return nil, fmt.Errorf("devopsjobs decode: %w", err)
		}
		se, ok := tok.(xml.StartElement)
		if !ok || !strings.EqualFold(se.Name.Local, "item") {
			continue
		}
		var it rssItem
		if err := dec.DecodeElement(&it, &se); err != nil {
			continue
		}
		titleRaw := strings.TrimSpace(it.Title)
		link := strings.TrimSpace(it.Link)
		if titleRaw == "" || link == "" {
			continue
		}
		if term != "" {
			hay := strings.ToLower(titleRaw + " " + it.Description)
			if !strings.Contains(hay, term) {
				continue
			}
		}
		parts := strings.Split(titleRaw, " - ")
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		jobTitle := titleRaw
		company := ""
		city := ""
		if len(parts) > 0 && parts[0] != "" {
			jobTitle = parts[0]
		}
		if len(parts) > 1 {
			company = parts[1]
		}
		if len(parts) > 2 {
			city = parts[2]
		}
		job := model.JobPost{
			ID:          "devopsjobs-" + idFromURL(firstNonEmpty(it.GUID, link)),
			Title:       jobTitle,
			CompanyName: company,
			JobURL:      link,
			Description: htmlToText(it.Description),
			Location:    model.Location{City: city},
		}
		if t := parseRSSDate(it.PubDate); t != nil {
			job.DatePosted = t
		}
		jobs = append(jobs, job)
		if len(jobs) >= wanted {
			break
		}
	}
	if !util.HasMeaningfulJobs(jobs) {
		return nil, fmt.Errorf("devopsjobs no parseable jobs")
	}
	return jobs, nil
}

func htmlToText(v string) string {
	if strings.TrimSpace(v) == "" {
		return ""
	}
	v = ddStripTags.ReplaceAllString(v, " ")
	return strings.Join(strings.Fields(strings.TrimSpace(v)), " ")
}

func parseRSSDate(v string) *time.Time {
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

func firstNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return strings.TrimSpace(a)
	}
	return strings.TrimSpace(b)
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
