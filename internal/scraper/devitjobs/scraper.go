package devitjobs

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

const feedURL = "https://devitjobs.com/job_feed.xml"

var stripTags = regexp.MustCompile(`(?is)<[^>]+>`)

type xmlItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	Company     string `xml:"company"`
	Location    string `xml:"location"`
	Salary      string `xml:"salary"`
	PubDate     string `xml:"pubDate"`
	Category    string `xml:"category"`
	Type        string `xml:"type"`
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

func (s *Scraper) SiteName() model.Site { return model.SiteDevITJobs }

func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	wanted := input.ResultsWanted
	if wanted <= 0 {
		wanted = 25
	}
	term := strings.ToLower(strings.TrimSpace(input.SearchTerm))

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, s.feedURL, nil)
	req.Header.Set("Accept", "application/xml, text/xml")
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("devitjobs request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("devitjobs status %d", resp.StatusCode)
	}

	body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("devitjobs read: %w", err)
	}

	type jobsEnvelope struct {
		Jobs  []xmlItem `xml:"job"`
		Items []xmlItem `xml:"item"`
	}
	var env jobsEnvelope
	if err := xml.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("devitjobs decode: %w", err)
	}
	rows := env.Jobs
	if len(rows) == 0 {
		rows = env.Items
	}
	jobs := make([]model.JobPost, 0, wanted)
	for _, it := range rows {
		title := strings.TrimSpace(it.Title)
		link := strings.TrimSpace(it.Link)
		if title == "" || link == "" {
			continue
		}
		if term != "" {
			hay := strings.ToLower(title + " " + it.Description + " " + it.Company + " " + it.Category)
			if !strings.Contains(hay, term) {
				continue
			}
		}
		job := model.JobPost{
			ID:          "devitjobs-" + idFromURL(link),
			Title:       title,
			CompanyName: strings.TrimSpace(it.Company),
			JobURL:      link,
			Description: htmlToText(it.Description),
			Location:    model.Location{City: strings.TrimSpace(it.Location), Country: "Switzerland"},
			IsRemote:    strings.Contains(strings.ToLower(it.Type+" "+it.Location), "remote"),
		}
		if t := parseRSSDate(it.PubDate); t != nil {
			job.DatePosted = t
		}
		job.Compensation = parseSalary(it.Salary)
		jobs = append(jobs, job)
		if len(jobs) >= wanted {
			break
		}
	}
	if !util.HasMeaningfulJobs(jobs) {
		return nil, fmt.Errorf("devitjobs no parseable jobs")
	}
	return jobs, nil
}

func parseSalary(v string) *model.Compensation {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	amounts := regexp.MustCompile(`[\d,']+`).FindAllString(v, -1)
	if len(amounts) == 0 {
		return nil
	}
	toInt := func(s string) *float64 {
		s = strings.ReplaceAll(s, ",", "")
		s = strings.ReplaceAll(s, "'", "")
		n, err := strconv.Atoi(s)
		if err != nil {
			return nil
		}
		f := float64(n)
		return &f
	}
	comp := &model.Compensation{Interval: model.IntervalYearly, Currency: "USD"}
	if strings.Contains(strings.ToLower(v), "eur") || strings.Contains(v, "€") {
		comp.Currency = "EUR"
	}
	if strings.Contains(strings.ToLower(v), "gbp") || strings.Contains(v, "£") {
		comp.Currency = "GBP"
	}
	if strings.Contains(strings.ToLower(v), "pln") || strings.Contains(strings.ToLower(v), "zł") {
		comp.Currency = "PLN"
	}
	if strings.Contains(strings.ToLower(v), "chf") {
		comp.Currency = "CHF"
	}
	if strings.Contains(strings.ToLower(v), "hour") || strings.Contains(strings.ToLower(v), "hr") {
		comp.Interval = model.IntervalHourly
	}
	comp.MinAmount = toInt(amounts[0])
	if len(amounts) > 1 {
		comp.MaxAmount = toInt(amounts[1])
	}
	return comp
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
