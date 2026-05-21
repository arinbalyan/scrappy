package wuzzuf

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

const defaultAPI = "https://wuzzuf.net/search/jobs/"

var reJobLink = regexp.MustCompile(`href="(https://wuzzuf\.net/jobs/p/[^"]+|/jobs/p/[^"]+)"`)

type Scraper struct {
	client *http.Client
	apiURL string
}

func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Retries: 3, Timeout: 18 * time.Second})
	}
	return &Scraper{client: client, apiURL: defaultAPI}
}
func NewWithAPIURL(client *http.Client, endpoint string) *Scraper {
	s := New(client)
	if strings.TrimSpace(endpoint) != "" {
		s.apiURL = endpoint
	}
	return s
}
func (s *Scraper) SiteName() model.Site { return model.SiteWuzzuf }

func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{"site": s.SiteName(), "results_wanted": input.ResultsWanted, "search_term": input.SearchTerm, "location": input.Location})

	u, _ := url.Parse(s.apiURL)
	q := u.Query()
	if strings.TrimSpace(input.SearchTerm) != "" {
		q.Set("q", strings.TrimSpace(input.SearchTerm))
	}
	u.RawQuery = q.Encode()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("wuzzuf request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("wuzzuf status %d", resp.StatusCode)
	}
	body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("wuzzuf read: %w", err)
	}

	jobs := parseAPIJobs(body)
	if !util.HasMeaningfulJobs(jobs) {
		jobs = parseHTMLJobs(string(body))
	}
	jobs = limitWuzzufJobs(jobs, input.ResultsWanted)
	if !util.HasMeaningfulJobs(jobs) {
		return nil, fmt.Errorf("wuzzuf no parseable jobs")
	}
	util.Debug("scraper_done", map[string]any{"site": s.SiteName(), "jobs": len(jobs)})
	return jobs, nil
}

func parseAPIJobs(body []byte) []model.JobPost {
	var parsed struct {
		Results []struct {
			ID, Title, URL, Description, PostedAt string
			Company                               struct {
				Name string `json:"name"`
			} `json:"company"`
			Location struct {
				Name string `json:"name"`
			} `json:"location"`
		} `json:"results"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil
	}
	jobs := make([]model.JobPost, 0, len(parsed.Results))
	for i, r := range parsed.Results {
		title := strings.TrimSpace(r.Title)
		company := strings.TrimSpace(r.Company.Name)
		if title == "" || company == "" {
			continue
		}
		post := model.JobPost{ID: "wz-" + strings.TrimSpace(r.ID), Title: title, CompanyName: company, Location: model.Location{City: strings.TrimSpace(r.Location.Name)}, JobURL: strings.TrimSpace(r.URL), Description: strings.TrimSpace(r.Description)}
		if post.ID == "wz-" {
			post.ID = fmt.Sprintf("wz-%d", i+1)
		}
		if t, err := time.Parse(time.RFC3339, r.PostedAt); err == nil {
			post.DatePosted = &t
		}
		jobs = append(jobs, post)
	}
	return jobs
}

func parseHTMLJobs(raw string) []model.JobPost {
	m := reJobLink.FindAllStringSubmatch(raw, -1)
	seen := make(map[string]struct{}, len(m))
	out := make([]model.JobPost, 0, len(m))
	for i, row := range m {
		jobURL := strings.TrimSpace(row[1])
		if strings.HasPrefix(jobURL, "/") {
			jobURL = "https://wuzzuf.net" + jobURL
		}
		if _, ok := seen[jobURL]; ok || jobURL == "" {
			continue
		}
		seen[jobURL] = struct{}{}
		slug := jobURL
		if idx := strings.LastIndex(slug, "/"); idx >= 0 && idx+1 < len(slug) {
			slug = slug[idx+1:]
		}
		out = append(out, model.JobPost{ID: fmt.Sprintf("wz-%s-%d", util.NormalizeSlug(slug), i+1), Title: "Wuzzuf Job", CompanyName: "Unknown Employer", JobURL: jobURL})
	}
	return out
}

func limitWuzzufJobs(in []model.JobPost, wanted int) []model.JobPost {
	if wanted <= 0 || wanted > len(in) {
		return in
	}
	return in[:wanted]
}
