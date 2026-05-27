package careeronestop

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

const defaultAPI = "https://api.careeronestop.org/v1/jobsearch"

// careerOneStopResponse wraps the API response.
type careerOneStopResponse struct {
	Jobs        []careerOneStopJob `json:"Jobs"`
	RecordCount int                `json:"RecordCount"`
}

type careerOneStopJob struct {
	JvId        string `json:"JvId"`
	Title       string `json:"Title"`
	Company     string `json:"Company"`
	URL         string `json:"URL"`
	Location    string `json:"Location"`
	Description string `json:"Description"`
	DatePosted  string `json:"DatePosted"`
}

// Scraper fetches jobs from the CareerOneStop API.
type Scraper struct {
	client *http.Client
	apiURL string
}

// New creates a new CareerOneStop scraper.
func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Timeout: 20 * time.Second})
	}
	return &Scraper{client: client, apiURL: defaultAPI}
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
func (s *Scraper) SiteName() model.Site { return model.SiteCareerOneStop }

// Scrape fetches jobs from the CareerOneStop API.
// NOTE: This API requires an API key. Set via CAREERONESTOP_API_KEY env var.
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

	keyword := strings.ReplaceAll(strings.TrimSpace(input.SearchTerm), " ", "%20")
	location := strings.ReplaceAll(strings.TrimSpace(input.Location), " ", "%20")
	radius := 50

	// CareerOneStop API path format:
	// /v1/jobsearch/{userId}/{keyword}/{location}/{radius}/{sortColumns}/{sortOrder}/{startRecord}/{pageSize}/{days}
	url := fmt.Sprintf("%s/anonymous/%s/%s/%d/relevance/asc/0/%d/0",
		s.apiURL, keyword, location, radius, wanted)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("careeronestop: build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("careeronestop: request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("careeronestop: status %d", resp.StatusCode)
	}

	body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("careeronestop: read: %w", err)
	}

	var apiResp careerOneStopResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("careeronestop: decode: %w", err)
	}

	if len(apiResp.Jobs) == 0 {
		return nil, fmt.Errorf("careeronestop: no jobs in response")
	}

	limit := wanted
	if limit > len(apiResp.Jobs) {
		limit = len(apiResp.Jobs)
	}

	out := make([]model.JobPost, 0, limit)
	for i := 0; i < limit; i++ {
		entry := apiResp.Jobs[i]
		title := strings.TrimSpace(entry.Title)
		jobURL := strings.TrimSpace(entry.URL)
		if title == "" || jobURL == "" {
			continue
		}

		desc := strings.TrimSpace(entry.Description)

		// Parse location "City, State"
		location := model.Location{}
		if locStr := strings.TrimSpace(entry.Location); locStr != "" {
			parts := strings.SplitN(locStr, ",", 2)
			location.City = strings.TrimSpace(parts[0])
			if len(parts) > 1 {
				location.State = strings.TrimSpace(parts[1])
			}
		}

		isRemote := strings.Contains(strings.ToLower(entry.Location), "remote")

		var datePosted *time.Time
		if dp := strings.TrimSpace(entry.DatePosted); dp != "" {
			datePosted = util.ParseDatePosted(dp)
		}

		job := model.JobPost{
			ID:          "careeronestop-" + entry.JvId,
			Title:       title,
			CompanyName: strings.TrimSpace(entry.Company),
			JobURL:      jobURL,
			Location:    location,
			Description: desc,
			DatePosted:  datePosted,
			IsRemote:    isRemote,
			Site:        string(s.SiteName()),
		}
		out = append(out, job)
	}

	util.Debug("scraper_done", map[string]any{
		"site": s.SiteName(),
		"jobs": len(out),
	})

	if !util.HasMeaningfulJobs(out) {
		return nil, fmt.Errorf("careeronestop: no parseable jobs")
	}
	return out, nil
}
