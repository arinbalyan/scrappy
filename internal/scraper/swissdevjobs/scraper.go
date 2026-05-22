package swissdevjobs

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/util"
)

const feedURL = "https://swissdevjobs.ch/rss"

var stripTags = regexp.MustCompile(`(?is)<[^>]+>`)

type rssItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	GUID        string `xml:"guid"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
	Category    string `xml:"category"`
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

func (s *Scraper) SiteName() model.Site { return model.SiteSwissDevJobs }

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
		return nil, fmt.Errorf("swissdevjobs request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("swissdevjobs status %d", resp.StatusCode)
	}

	dec := xml.NewDecoder(resp.Body)
	jobs := make([]model.JobPost, 0, wanted)
	for {
		tok, err := dec.Token()
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			return nil, fmt.Errorf("swissdevjobs decode: %w", err)
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
		if term != "" {
			hay := strings.ToLower(title + " " + it.Description + " " + it.Category)
			if !strings.Contains(hay, term) {
				continue
			}
		}
		job := model.JobPost{
			ID:          "swissdevjobs-" + idFromURL(firstNonEmpty(it.GUID, link)),
			Title:       title,
			CompanyName: companyFromTitle(title),
			JobURL:      link,
			Description: htmlToText(it.Description),
			Location:    model.Location{Country: "Switzerland"},
		}
		if t := parseRSSDate(it.PubDate); t != nil {
			job.DatePosted = t
		}
		job.Compensation = salaryFromTitle(title)
		jobs = append(jobs, job)
		if len(jobs) >= wanted {
			break
		}
	}
	if !util.HasMeaningfulJobs(jobs) {
		return nil, fmt.Errorf("swissdevjobs no parseable jobs")
	}
	return jobs, nil
}

func salaryFromTitle(title string) *model.Compensation {
	re := regexp.MustCompile(`\[(?:CHF|EUR)\s*([\d']+)\s*-\s*([\d']+)\]`)
	m := re.FindStringSubmatch(title)
	if len(m) != 3 {
		return nil
	}
	toAmount := func(v string) *float64 {
		n, err := strconv.Atoi(strings.ReplaceAll(v, "'", ""))
		if err != nil {
			return nil
		}
		f := float64(n)
		return &f
	}
	currency := "CHF"
	if strings.Contains(title, "[EUR") {
		currency = "EUR"
	}
	return &model.Compensation{
		Interval:  model.IntervalYearly,
		MinAmount: toAmount(m[1]),
		MaxAmount: toAmount(m[2]),
		Currency:  currency,
	}
}

func companyFromTitle(title string) string {
	re := regexp.MustCompile(`@\s*(.+?)\s*\[`)
	m := re.FindStringSubmatch(title)
	if len(m) == 2 {
		return strings.TrimSpace(m[1])
	}
	return ""
}

func htmlToText(v string) string {
	if strings.TrimSpace(v) == "" {
		return ""
	}
	v = stripTags.ReplaceAllString(v, " ")
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
	if t, err := time.Parse(time.RFC3339, v); err == nil {
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
