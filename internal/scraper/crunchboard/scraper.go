package crunchboard

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

const feedURL = "https://www.crunchboard.com/jobs.rss"

var cbStripTags = regexp.MustCompile(`(?is)<[^>]+>`)

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

func (s *Scraper) SiteName() model.Site { return model.SiteCrunchboard }

func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	wanted := input.ResultsWanted
	if wanted <= 0 {
		wanted = 25
	}
	terms := parseSearchTerms(input.SearchTerm)

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, s.feedURL, nil)
	req.Header.Set("Accept", "application/rss+xml, application/xml, text/xml")
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("crunchboard request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("crunchboard status %d", resp.StatusCode)
	}

	dec := xml.NewDecoder(resp.Body)
	jobs := make([]model.JobPost, 0, wanted)
	for {
		tok, err := dec.Token()
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			return nil, fmt.Errorf("crunchboard decode: %w", err)
		}
		se, ok := tok.(xml.StartElement)
		if !ok || !strings.EqualFold(se.Name.Local, "item") {
			continue
		}
		var it rssItem
		if err := dec.DecodeElement(&it, &se); err != nil {
			continue
		}
		title := strings.TrimSpace(it.Title)
		link := strings.TrimSpace(it.Link)
		if title == "" || link == "" {
			continue
		}
		if len(terms) > 0 {
			hay := strings.ToLower(title + " " + it.Description)
			if !matchAny(hay, terms) {
				continue
			}
		}

		job := model.JobPost{
			ID:          "crunchboard-" + idFromURL(firstNonEmpty(it.GUID, link)),
			Title:       title,
			CompanyName: "",
			JobURL:      link,
			Description: htmlToText(it.Description),
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
		return nil, fmt.Errorf("crunchboard no parseable jobs")
	}
	return jobs, nil
}

func htmlToText(v string) string {
	if strings.TrimSpace(v) == "" {
		return ""
	}
	v = cbStripTags.ReplaceAllString(v, " ")
	return strings.Join(strings.Fields(strings.TrimSpace(v)), " ")
}

// parseSearchTerms splits a search term on " OR " and returns lowercase terms.
func parseSearchTerms(raw string) []string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return nil
	}
	parts := strings.Split(s, " OR ")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.Trim(strings.TrimSpace(p), `"`)
		if p != "" {
			out = append(out, strings.ToLower(p))
		}
	}
	return out
}

// matchAny returns true if the haystack contains any of the terms.
func matchAny(hay string, terms []string) bool {
	for _, t := range terms {
		if strings.Contains(hay, t) {
			return true
		}
	}
	return false
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
