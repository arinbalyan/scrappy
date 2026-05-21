package bdjobs

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

const (
	defaultListURL = "https://bdjobs.com/h/jobs?lang=en"
)

var reJobLink = regexp.MustCompile(`(?is)<a[^>]*href="(https?://[^"]*?/jobdetails[^"]*)"[^>]*>\s*([^<]{4,180})\s*</a>`)

type Scraper struct {
	Client  *http.Client
	ListURL string
}

func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Retries: 3, Timeout: 20 * time.Second})
	}
	return &Scraper{Client: client, ListURL: defaultListURL}
}

func NewWithListURL(client *http.Client, u string) *Scraper {
	s := New(client)
	if strings.TrimSpace(u) != "" {
		s.ListURL = u
	}
	return s
}

func (s *Scraper) SiteName() model.Site { return model.SiteBDJobs }

func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	searchURL := s.buildSearchURL(input)
	util.Debug("scraper_start", map[string]any{"site": s.SiteName(), "results_wanted": input.ResultsWanted})

	if input.SearchTerm != "" {
		jobs, err := s.scrapePage(ctx, searchURL, input)
		if err == nil && util.HasMeaningfulJobs(jobs) {
			return limitJobs(jobs, input.ResultsWanted), nil
		}
	}

	jobs, err := s.scrapePage(ctx, s.ListURL, input)
	if err != nil && err.Error() == "" {
		err = fmt.Errorf("bdjobs no parseable jobs")
	}
	if !util.HasMeaningfulJobs(jobs) {
		return nil, fmt.Errorf("bdjobs no parseable jobs: %v", err)
	}
	return limitJobs(jobs, input.ResultsWanted), nil
}

func (s *Scraper) buildSearchURL(input model.ScraperInput) string {
	parts := make([]string, 0, 2)
	if input.SearchTerm != "" {
		parts = append(parts, "keywords="+strings.ReplaceAll(input.SearchTerm, " ", "+"))
	}
	if input.Location != "" {
		parts = append(parts, "location="+strings.ReplaceAll(input.Location, " ", "+"))
	}
	if len(parts) == 0 {
		return defaultListURL
	}
	return "https://bdjobs.com/h/jobs?" + strings.Join(parts, "&") + "&lang=en"
}

func (s *Scraper) scrapePage(ctx context.Context, targetURL string, input model.ScraperInput) ([]model.JobPost, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Accept", "text/html")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	resp, err := s.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("bdjobs request: %w", err)
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusForbidden, http.StatusNotFound:
		return nil, fmt.Errorf("bdjobs blocked status %d", resp.StatusCode)
	case http.StatusTooManyRequests:
		return nil, fmt.Errorf("bdjobs rate limited status 429")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("bdjobs status %d", resp.StatusCode)
	}
	b, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("bdjobs read: %w", err)
	}
	body := string(b)

	if strings.Contains(strings.ToLower(body), "captcha") || strings.Contains(strings.ToLower(body), "access denied") {
		return nil, fmt.Errorf("bdjobs challenge detected")
	}

	out := make([]model.JobPost, 0, 64)
	seen := map[string]struct{}{}

	// Use innerHTML capture then stripTags — handles both bare text and tag-wrapped links
	re2 := regexp.MustCompile(`(?is)<a[^>]*href="(https?://[^"]*?/jobdetails[^"]*)"[^>]*>([\s\S]*?)</a>`)
	for i, row := range re2.FindAllStringSubmatch(body, -1) {
		if input.ResultsWanted > 0 && len(out) >= input.ResultsWanted {
			break
		}
		u := strings.TrimSpace(row[1])
		if strings.HasPrefix(u, "/") {
			u = "https://bdjobs.com" + u
		}
		t := strings.TrimSpace(regexp.MustCompile(`<[^>]+>`).ReplaceAllString(row[2], " "))
		t = regexp.MustCompile(`\s+`).ReplaceAllString(t, " ")
		if u == "" || t == "" {
			continue
		}
		if _, ok := seen[u]; ok {
			continue
		}
		seen[u] = struct{}{}
		out = append(out, model.JobPost{
			ID:      fmt.Sprintf("bdjobs-%d", i+1),
			Title:   t,
			JobURL:  u,
		})
	}

	if !util.HasMeaningfulJobs(out) {
		return nil, fmt.Errorf("bdjobs no parseable jobs")
	}
	return out, nil
}

func limitJobs(in []model.JobPost, wanted int) []model.JobPost {
	if wanted <= 0 || wanted > len(in) {
		return in
	}
	return in[:wanted]
}
