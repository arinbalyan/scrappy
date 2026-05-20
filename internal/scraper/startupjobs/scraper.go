package startupjobs

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/util"
)

const defaultAPI = "https://startup.jobs/api/job"

type Scraper struct {
	client *http.Client
	apiURL string
}

func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Retries: 3, CookieResetEveryN: 140, Timeout: 18 * time.Second})
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

func (s *Scraper) SiteName() model.Site { return model.SiteStartupJobs }

func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, s.apiURL, nil)
	req.Header.Set("Accept", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("startupjobs request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("startupjobs status %d", resp.StatusCode)
	}

	var parsed []struct {
		ID          string `json:"id"`
		Title       string `json:"title"`
		CompanyName string `json:"company_name"`
		Description string `json:"description"`
		Remote      bool   `json:"remote"`
		Location    string `json:"location"`
		CreatedAt   string `json:"created_at"`
		URL         string `json:"url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("startupjobs decode: %w", err)
	}
	limit := input.ResultsWanted
	if limit <= 0 || limit > len(parsed) {
		limit = len(parsed)
	}
	jobs := make([]model.JobPost, 0, limit)
	for i := 0; i < limit; i++ {
		r := parsed[i]
		var posted *time.Time
		if t, err := time.Parse(time.RFC3339, r.CreatedAt); err == nil {
			posted = &t
		}
		jobs = append(jobs, model.JobPost{ID: "stj-" + strings.TrimSpace(r.ID), Title: strings.TrimSpace(r.Title), CompanyName: strings.TrimSpace(r.CompanyName), JobURL: strings.TrimSpace(r.URL), Description: strings.TrimSpace(r.Description), Location: model.Location{City: strings.TrimSpace(r.Location)}, DatePosted: posted, IsRemote: r.Remote || strings.Contains(strings.ToLower(r.Location), "remote")})
	}
	return jobs, nil
}
