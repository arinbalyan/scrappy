package otta

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/util"
)

const (
	defaultHomeURL = "https://otta.com/"
	defaultSearch  = "https://otta.com/en/jobs"
)

var reJobSearch = regexp.MustCompile(`(?is)<a[^>]*href="([^"]*?/[a-z]{2}/jobs/[^"]*)"[^>]*>\s*([^<]{4,160})\s*</a>`)
var reLDJSON   = regexp.MustCompile(`(?is)<script[^>]*type="application/ld\+json"[^>]*>(.*?)</script>`)
var reCtrlChars = regexp.MustCompile(`[\x00-\x1f]`)

type Scraper struct {
	Client  *http.Client
	ListURL string
}

func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Retries: 3, Timeout: 20 * time.Second})
	}
	return &Scraper{Client: client, ListURL: defaultHomeURL}
}

func NewWithListURL(client *http.Client, u string) *Scraper {
	s := New(client)
	if strings.TrimSpace(u) != "" {
		s.ListURL = u
	}
	return s
}

func (s *Scraper) SiteName() model.Site { return model.SiteOtta }

func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{"site": s.SiteName(), "results_wanted": input.ResultsWanted})

	targetURL := s.buildSearchURL(input)

	// Try search/listing page first
	jobs, err := s.scrapePage(ctx, targetURL, input)
	if err == nil && util.HasMeaningfulJobs(jobs) {
		return limitJobs(jobs, input.ResultsWanted), nil
	}

	// Fallback to homepage
	if targetURL != s.ListURL {
		jobs, err = s.scrapePage(ctx, s.ListURL, input)
		if err == nil && util.HasMeaningfulJobs(jobs) {
			return limitJobs(jobs, input.ResultsWanted), nil
		}
	}

	if err != nil {
		return nil, fmt.Errorf("otta: %w", err)
	}
	return nil, fmt.Errorf("otta no parseable jobs")
}

func (s *Scraper) buildSearchURL(input model.ScraperInput) string {
	u, err := url.Parse(defaultSearch)
	if err != nil {
		return defaultSearch
	}
	q := u.Query()
	if input.SearchTerm != "" {
		q.Set("search", input.SearchTerm)
	}
	if input.Location != "" {
		q.Set("location", input.Location)
	}
	if input.IsRemote {
		q.Set("remote", "true")
	}
	u.RawQuery = q.Encode()
	return u.String()
}

func (s *Scraper) scrapePage(ctx context.Context, targetURL string, input model.ScraperInput) ([]model.JobPost, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Accept", "text/html")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	resp, err := s.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("otta request: %w", err)
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusForbidden:
		return nil, fmt.Errorf("otta blocked status 403")
	case http.StatusNotFound:
		return nil, fmt.Errorf("otta page not found status 404")
	case http.StatusTooManyRequests:
		return nil, fmt.Errorf("otta rate limited status 429")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("otta status %d", resp.StatusCode)
	}
	b, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("otta read: %w", err)
	}

	if strings.Contains(strings.ToLower(string(b)), "captcha") || strings.Contains(strings.ToLower(string(b)), "attention required") ||
		strings.Contains(strings.ToLower(string(b)), "cloudflare") {
		return nil, fmt.Errorf("otta challenge detected")
	}
	body := string(b)
	if jobs := parseLDJSONJobs(body); util.HasMeaningfulJobs(jobs) {
		return jobs, nil
	}
	return parseOttaHTML(body, input)
}

func parseOttaHTML(raw string, input model.ScraperInput) ([]model.JobPost, error) {
	out := make([]model.JobPost, 0, 64)
	seen := map[string]struct{}{}

	// Strategy 1: search listing links (/en/jobs/pattern)
	m := reJobSearch.FindAllStringSubmatch(raw, -1)
	for i, row := range m {
		if input.ResultsWanted > 0 && len(out) >= input.ResultsWanted {
			break
		}
		u := row[1]
		if !strings.HasPrefix(u, "http") {
			if strings.HasPrefix(u, "/") {
				u = "https://otta.com" + u
			} else {
				continue
			}
		}
		t := strings.TrimSpace(regexp.MustCompile(`\s+`).ReplaceAllString(row[2], " "))
		if t == "" {
			continue
		}
		if _, ok := seen[u]; ok {
			continue
		}
		seen[u] = struct{}{}
		out = append(out, model.JobPost{
			ID:      fmt.Sprintf("otta-%d", i+1),
			Title:   t,
			JobURL:  u,
			IsRemote: strings.Contains(strings.ToLower(t), "remote"),
		})
	}

	if !util.HasMeaningfulJobs(out) {
		return nil, fmt.Errorf("otta no parseable jobs")
	}
	return out, nil
}

type ldJob struct {
	Type        string `json:"@type"`
	Title       string `json:"title"`
	Description string `json:"description"`
	DatePosted  string `json:"datePosted"`
	URL         string `json:"url"`
	HiringOrg   struct {
		Name string `json:"name"`
	} `json:"hiringOrganization"`
}
type ldGraph struct {
	Graph []ldJob `json:"@graph"`
}

func parseLDJSONJobs(raw string) []model.JobPost {
	cleaned := reCtrlChars.ReplaceAllString(raw, " ")
	for i, m := range reLDJSON.FindAllStringSubmatch(cleaned, -1) {
		_ = i
		body := strings.TrimSpace(m[1])
		if body == "" {
			continue
		}
		var job ldJob
		if err := json.Unmarshal([]byte(body), &job); err == nil && job.Type == "JobPosting" {
			return []model.JobPost{toPost(job, i)}
		}
		var arr []ldJob
		if err := json.Unmarshal([]byte(body), &arr); err == nil {
			return ldJobsToPosts(arr, i)
		}
		var graph ldGraph
		if err := json.Unmarshal([]byte(body), &graph); err == nil {
			return ldJobsToPosts(graph.Graph, i)
		}
	}
	return nil
}

func ldJobsToPosts(jobs []ldJob, seedIdx int) []model.JobPost {
	out := make([]model.JobPost, 0, len(jobs))
	for idx, j := range jobs {
		if j.Type == "JobPosting" {
			out = append(out, toPost(j, seedIdx*1000+idx))
		}
	}
	return out
}

func toPost(j ldJob, seed int) model.JobPost {
	title := strings.TrimSpace(j.Title)
	company := strings.TrimSpace(j.HiringOrg.Name)
	post := model.JobPost{
		ID:          fmt.Sprintf("otta-%s-%s-%d", util.NormalizeSlug(title), util.NormalizeSlug(company), seed),
		Title:       title,
		CompanyName: company,
		JobURL:      strings.TrimSpace(j.URL),
	}
	if post.JobURL == "" {
		post.JobURL = defaultHomeURL
	}
	if desc := strings.TrimSpace(j.Description); desc != "" {
		post.Description = desc
	}
	if j.DatePosted != "" {
		if t, err := time.Parse(time.RFC3339, j.DatePosted); err == nil {
			post.DatePosted = &t
		} else if t, err2 := time.Parse("2006-01-02", j.DatePosted); err2 == nil {
			post.DatePosted = &t
		}
	}
	return post
}

func limitJobs(in []model.JobPost, wanted int) []model.JobPost {
	if wanted <= 0 || wanted > len(in) {
		return in
	}
	return in[:wanted]
}
