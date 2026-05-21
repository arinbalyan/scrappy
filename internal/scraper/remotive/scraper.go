package remotive

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/util"
)

const defaultAPI = "https://remotive.com/api/remote-jobs"

type Scraper struct {
	client *http.Client
	apiURL string
}

func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Retries: 2, CookieResetEveryN: 160, Timeout: 15 * time.Second})
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

func (s *Scraper) SiteName() model.Site { return model.SiteRemotive }

func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{"site": s.SiteName(), "results_wanted": input.ResultsWanted, "search_term": input.SearchTerm, "location": input.Location})

	u, _ := url.Parse(s.apiURL)
	q := u.Query()
	if input.SearchTerm != "" {
		q.Set("search", input.SearchTerm)
	}
	u.RawQuery = q.Encode()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("remotive request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("remotive status %d", resp.StatusCode)
	}

	var parsed struct {
		Jobs []struct {
			ID                        int    `json:"id"`
			Title                     string `json:"title"`
			CompanyName               string `json:"company_name"`
			Category                  string `json:"category"`
			CandidateRequiredLocation string `json:"candidate_required_location"`
			PublicationDate           string `json:"publication_date"`
			URL                       string `json:"url"`
			Salary                    string `json:"salary"`
			Description               string `json:"description"`
		} `json:"jobs"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("remotive decode: %w", err)
	}

	limit := input.ResultsWanted
	if limit <= 0 || limit > len(parsed.Jobs) {
		limit = len(parsed.Jobs)
	}
	jobs := make([]model.JobPost, 0, limit)
	for i := 0; i < limit; i++ {
		j := parsed.Jobs[i]
		var posted *time.Time
		if t, err := time.Parse(time.RFC3339, j.PublicationDate); err == nil {
			posted = &t
		}
		jobs = append(jobs, model.JobPost{
			ID:          fmt.Sprintf("re-%d", j.ID),
			Title:       j.Title,
			CompanyName: j.CompanyName,
			Location:    model.Location{City: j.CandidateRequiredLocation},
			IsRemote:    true,
			DatePosted:  posted,
			JobURL:      j.URL,
			Description: j.Description,
			JobType:     strings.ToLower(j.Category),
		})
	}
	util.Debug("scraper_done", map[string]any{"site": s.SiteName(), "jobs": len(jobs)})
	if !util.HasMeaningfulJobs(jobs) {
		return nil, fmt.Errorf("remotive no parseable jobs")
	}
	return jobs, nil
}
