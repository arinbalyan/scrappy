package adzuna

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/util"
)

const defaultAPI = "https://api.adzuna.com/v1/api/jobs/us/search/1"

type Scraper struct {
	client *http.Client
	apiURL string
}

func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Retries: 3, CookieResetEveryN: 160, Timeout: 18 * time.Second})
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

func (s *Scraper) SiteName() model.Site { return model.SiteAdzuna }

func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	appID := strings.TrimSpace(input.AdzunaAppID)
	if appID == "" {
		appID = strings.TrimSpace(os.Getenv("SCRAPPY_ADZUNA_APP_ID"))
	}
	appKey := strings.TrimSpace(input.AdzunaAppKey)
	if appKey == "" {
		appKey = strings.TrimSpace(os.Getenv("SCRAPPY_ADZUNA_APP_KEY"))
	}
	if appID == "" || appKey == "" {
		util.APIMiss("adzuna_missing_credentials", map[string]any{"site": model.SiteAdzuna})
		return nil, nil
	}

	u, _ := url.Parse(s.apiURL)
	q := u.Query()
	q.Set("app_id", appID)
	q.Set("app_key", appKey)
	if input.SearchTerm != "" {
		q.Set("what", input.SearchTerm)
	}
	if input.Location != "" {
		q.Set("where", input.Location)
	}
	u.RawQuery = q.Encode()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	req.Header.Set("Accept", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("adzuna request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("adzuna status %d", resp.StatusCode)
	}

	var parsed struct {
		Results []struct {
			ID          string `json:"id"`
			Title       string `json:"title"`
			Description string `json:"description"`
			RedirectURL string `json:"redirect_url"`
			Created     string `json:"created"`
			Company     struct {
				DisplayName string `json:"display_name"`
			} `json:"company"`
			Location struct {
				DisplayName string `json:"display_name"`
			} `json:"location"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("adzuna decode: %w", err)
	}

	limit := input.ResultsWanted
	if limit <= 0 || limit > len(parsed.Results) {
		limit = len(parsed.Results)
	}
	jobs := make([]model.JobPost, 0, limit)
	for i := 0; i < limit; i++ {
		r := parsed.Results[i]
		var posted *time.Time
		if t, err := time.Parse(time.RFC3339, r.Created); err == nil {
			posted = &t
		}
		jobs = append(jobs, model.JobPost{
			ID:          fmt.Sprintf("adz-%s", strings.TrimSpace(r.ID)),
			Title:       strings.TrimSpace(r.Title),
			CompanyName: strings.TrimSpace(r.Company.DisplayName),
			JobURL:      strings.TrimSpace(r.RedirectURL),
			Description: strings.TrimSpace(r.Description),
			Location:    model.Location{City: strings.TrimSpace(r.Location.DisplayName)},
			DatePosted:  posted,
			IsRemote:    strings.Contains(strings.ToLower(r.Location.DisplayName), "remote"),
		})
	}
	return jobs, nil
}
