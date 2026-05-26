package duunitori

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

const apiURL = "https://duunitori.fi/api/v1/jobs"

// duunitoriApiResponse wraps the API response.
type duunitoriApiResponse struct {
	Results []duunitoriJobEntry `json:"results"`
	Count   int                 `json:"count"`
}

type duunitoriJobEntry struct {
	Slug            string `json:"slug"`
	Heading         string `json:"heading"`
	CompanyName     string `json:"company_name"`
	MunicipalityName string `json:"municipality_name"`
	Descr           string `json:"descr"`
	DatePosted      string `json:"date_posted"`
}

// Scraper fetches jobs from the Duunitori API.
type Scraper struct {
	client *http.Client
	apiURL string
}

// New creates a new Duunitori scraper.
func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Timeout: 20 * time.Second})
	}
	return &Scraper{client: client, apiURL: apiURL}
}

// NewWithAPIURL creates a scraper with a custom API URL (used in tests).
func NewWithAPIURL(client *http.Client, endpoint string) *Scraper {
	s := New(client)
	if strings.TrimSpace(endpoint) != "" {
		s.apiURL = endpoint
	}
	return s
}

// SiteName returns the site identifier.
func (s *Scraper) SiteName() model.Site { return model.SiteDuunitori }

// Scrape fetches jobs from the Duunitori API.
func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{
		"site":           s.SiteName(),
		"results_wanted": input.ResultsWanted,
		"search_term":    input.SearchTerm,
	})

	wanted := input.ResultsWanted
	if wanted <= 0 {
		wanted = 25
	}

	queryParams := fmt.Sprintf("format=json&page=1&page_size=%d", wanted)
	if term := strings.TrimSpace(input.SearchTerm); term != "" {
		queryParams += "&search=" + strings.ReplaceAll(term, " ", "+")
	}

	url := s.apiURL + "?" + queryParams

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("duunitori: build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("duunitori: request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("duunitori: status %d", resp.StatusCode)
	}

	body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("duunitori: read: %w", err)
	}

	var apiResp duunitoriApiResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("duunitori: decode: %w", err)
	}

	if len(apiResp.Results) == 0 {
		return nil, fmt.Errorf("duunitori: no jobs in response")
	}

	limit := wanted
	if limit > len(apiResp.Results) {
		limit = len(apiResp.Results)
	}

	out := make([]model.JobPost, 0, limit)
	for i := 0; i < limit; i++ {
		r := apiResp.Results[i]
		title := strings.TrimSpace(r.Heading)
		slug := strings.TrimSpace(r.Slug)
		if title == "" || slug == "" {
			continue
		}

		desc := strings.TrimSpace(r.Descr)

		location := model.Location{
			City:    strings.TrimSpace(r.MunicipalityName),
			Country: "Finland",
		}

		jobURL := fmt.Sprintf("https://duunitori.fi/tyopaikat/%s", slug)

		var datePosted *time.Time
		if dp := strings.TrimSpace(r.DatePosted); dp != "" {
			datePosted = util.ParseDatePosted(dp)
		}

		job := model.JobPost{
			ID:          "duunitori-" + slug,
			Title:       title,
			CompanyName: strings.TrimSpace(r.CompanyName),
			JobURL:      jobURL,
			Location:    location,
			Description: desc,
			DatePosted:  datePosted,
			Site:        string(s.SiteName()),
		}
		out = append(out, job)
	}

	util.Debug("scraper_done", map[string]any{
		"site": s.SiteName(),
		"jobs": len(out),
	})

	if !util.HasMeaningfulJobs(out) {
		return nil, fmt.Errorf("duunitori: no parseable jobs")
	}
	return out, nil
}
