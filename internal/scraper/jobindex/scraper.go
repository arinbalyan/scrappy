package jobindex

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

const defaultAPI = "https://www.jobindex.dk/jobsoegning"

var reStash = regexp.MustCompile(`(?s)var\s+Stash\s*=\s*(\{.*?\})\s*;`)

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
func (s *Scraper) SiteName() model.Site { return model.SiteJobindex }

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
		return nil, fmt.Errorf("jobindex request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("jobindex status %d", resp.StatusCode)
	}
	body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("jobindex read: %w", err)
	}

	jobs := parseAPIJobs(body)
	if !util.HasMeaningfulJobs(jobs) {
		jobs = parseFromStash(string(body))
	}
	jobs = limitJobs(jobs, input.ResultsWanted)
	if !util.HasMeaningfulJobs(jobs) {
		return nil, nil
	}
	util.Debug("scraper_done", map[string]any{"site": s.SiteName(), "jobs": len(jobs)})
	return jobs, nil
}

func parseAPIJobs(body []byte) []model.JobPost {
	var parsed []struct{ ID, Title, Company, Location, URL, Description, PostedAt string }
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil
	}
	out := make([]model.JobPost, 0, len(parsed))
	for i, r := range parsed {
		title := strings.TrimSpace(r.Title)
		company := strings.TrimSpace(r.Company)
		if title == "" || company == "" {
			continue
		}
		post := model.JobPost{ID: "ji-" + strings.TrimSpace(r.ID), Title: title, CompanyName: company, Location: model.Location{City: strings.TrimSpace(r.Location)}, JobURL: strings.TrimSpace(r.URL), Description: strings.TrimSpace(r.Description)}
		if post.ID == "ji-" {
			post.ID = fmt.Sprintf("ji-%d", i+1)
		}
		if post.JobURL == "" {
			post.JobURL = "https://www.jobindex.dk"
		}
		if t, err := time.Parse(time.RFC3339, strings.TrimSpace(r.PostedAt)); err == nil {
			post.DatePosted = &t
		}
		out = append(out, post)
	}
	return out
}

func parseFromStash(raw string) []model.JobPost {
	m := reStash.FindStringSubmatch(raw)
	if len(m) < 2 {
		return nil
	}
	var parsed struct {
		JobSearch struct {
			StoreData struct {
				SearchResponse struct {
					Results []struct {
						Tid       string `json:"tid"`
						Headline  string `json:"headline"`
						Company   string `json:"companytext"`
						ShareURL  string `json:"share_url"`
						FirstDate string `json:"firstdate"`
						Area      string `json:"area"`
						HTML      string `json:"html"`
					} `json:"results"`
				} `json:"searchResponse"`
			} `json:"storeData"`
		} `json:"jobsearch/result_app"`
	}
	if err := json.Unmarshal([]byte(m[1]), &parsed); err != nil {
		return nil
	}
	rows := parsed.JobSearch.StoreData.SearchResponse.Results
	out := make([]model.JobPost, 0, len(rows))
	for i, r := range rows {
		title := strings.TrimSpace(r.Headline)
		company := strings.TrimSpace(r.Company)
		if title == "" || company == "" {
			continue
		}
		jobURL := strings.TrimSpace(r.ShareURL)
		if jobURL == "" {
			jobURL = "https://www.jobindex.dk"
		}
		post := model.JobPost{
			ID:          "ji-" + strings.TrimSpace(r.Tid),
			Title:       title,
			CompanyName: company,
			Location:    model.Location{City: strings.TrimSpace(r.Area)},
			JobURL:      jobURL,
			Description: strings.TrimSpace(stripTags(r.HTML)),
		}
		if post.ID == "ji-" {
			post.ID = fmt.Sprintf("ji-%d", i+1)
		}
		if t, err := time.Parse("2006-01-02", strings.TrimSpace(r.FirstDate)); err == nil {
			post.DatePosted = &t
		}
		out = append(out, post)
	}
	return out
}

func stripTags(s string) string {
	re := regexp.MustCompile(`<[^>]+>`)
	clean := re.ReplaceAllString(s, " ")
	return strings.Join(strings.Fields(clean), " ")
}

func limitJobs(in []model.JobPost, wanted int) []model.JobPost {
	if wanted <= 0 || wanted > len(in) {
		return in
	}
	return in[:wanted]
}
