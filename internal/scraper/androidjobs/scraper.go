package androidjobs

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

const feedURL = "https://androidjobs.io/jobs.rss"

var ajStripTags = regexp.MustCompile(`(?is)<[^>]+>`)

type rssDoc struct {
	Items []rssItem `xml:"channel>item"`
}

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
		client = util.NewHTTPClient(util.ClientOptions{Retries: 2, Timeout: 15 * time.Second})
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

func (s *Scraper) SiteName() model.Site { return model.SiteAndroidJobs }

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
		return nil, fmt.Errorf("androidjobs request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("androidjobs status %d", resp.StatusCode)
	}

	body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("androidjobs read: %w", err)
	}

	var doc rssDoc
	if err := xml.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("androidjobs decode: %w", err)
	}

	jobs := make([]model.JobPost, 0, wanted)
	for _, item := range doc.Items {
		if len(jobs) >= wanted {
			break
		}
		titleRaw := strings.TrimSpace(item.Title)
		link := strings.TrimSpace(item.Link)
		if titleRaw == "" || link == "" {
			continue
		}
		if term != "" {
			hay := strings.ToLower(titleRaw + " " + item.Description)
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

		idPart := strings.TrimSpace(item.GUID)
		if idPart == "" {
			idPart = idFromURL(link)
		}
		job := model.JobPost{
			ID:          "androidjobs-" + idPart,
			Title:       jobTitle,
			CompanyName: company,
			JobURL:      link,
			Description: htmlToText(item.Description),
			Location: model.Location{
				City: city,
			},
		}
		if t := parseRSSDate(item.PubDate); t != nil {
			job.DatePosted = t
		}
		jobs = append(jobs, job)
	}

	if !util.HasMeaningfulJobs(jobs) {
		return nil, fmt.Errorf("androidjobs no parseable jobs")
	}
	return jobs, nil
}

func htmlToText(v string) string {
	if strings.TrimSpace(v) == "" {
		return ""
	}
	v = ajStripTags.ReplaceAllString(v, " ")
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
