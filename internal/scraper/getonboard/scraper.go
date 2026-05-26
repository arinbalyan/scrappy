package getonboard

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/util"
)

const defaultAPI = "https://www.getonbrd.com/api/v0/search/jobs"

// Scraper fetches jobs from the GetOnBoard public API.
type Scraper struct {
	client *http.Client
	apiURL string
}

// New creates a new GetOnBoard scraper. If client is nil a default one is used.
func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Timeout: 20 * time.Second})
	}
	return &Scraper{client: client, apiURL: defaultAPI}
}

// NewWithAPIURL creates a new scraper with a custom endpoint (used in tests).
func NewWithAPIURL(client *http.Client, endpoint string) *Scraper {
	s := New(client)
	if strings.TrimSpace(endpoint) != "" {
		s.apiURL = endpoint
	}
	return s
}

// SiteName returns the site identifier.
func (s *Scraper) SiteName() model.Site { return model.SiteGetOnBoard }

// --- API response types ---

type getOnBoardSearchResponse struct {
	Data []getOnBoardJob    `json:"data"`
	Meta *getOnBoardMeta    `json:"meta,omitempty"`
}

type getOnBoardMeta struct {
	CurrentPage int `json:"current_page"`
	TotalPages  int `json:"total_pages"`
	TotalResults int `json:"total_results"`
}

type getOnBoardJob struct {
	ID         string               `json:"id"`
	Type       string               `json:"type"`
	Attributes getOnBoardAttributes `json:"attributes"`
	Links      getOnBoardLinks      `json:"links"`
}

type getOnBoardAttributes struct {
	Title       string           `json:"title"`
	Description string           `json:"description"`
	Company     string           `json:"company"`
	Logo        string           `json:"logo"`
	MinSalary   *float64         `json:"min_salary"`
	MaxSalary   *float64         `json:"max_salary"`
	Remote      bool             `json:"remote"`
	Seniority   json.RawMessage  `json:"seniority"`
	PublishedAt *int64           `json:"published_at"`
	Countries   []string         `json:"countries"`
	LocationCities json.RawMessage `json:"location_cities"`
	Tags        []string         `json:"tags"`
}

type getOnBoardLinks struct {
	PublicURL string `json:"public_url"`
}

// Scrape fetches jobs from GetOnBoard. Single-page, up to 50 results.
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
	if wanted > 100 {
		wanted = 100
	}

	u, _ := url.Parse(s.apiURL)
	q := url.Values{}
	q.Set("per_page", strconv.Itoa(minInt(wanted, 50)))
	q.Set("page", "1")
	if input.SearchTerm != "" {
		q.Set("query", input.SearchTerm)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("getonboard: build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("getonboard: request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("getonboard: status %d", resp.StatusCode)
	}

	body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("getonboard: read: %w", err)
	}

	var apiResp getOnBoardSearchResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("getonboard: decode: %w", err)
	}

	if len(apiResp.Data) == 0 {
		return nil, fmt.Errorf("getonboard: no jobs returned")
	}

	out := make([]model.JobPost, 0, wanted)
	for _, raw := range apiResp.Data {
		if len(out) >= wanted {
			break
		}

		attrs := raw.Attributes
		title := strings.TrimSpace(attrs.Title)
		if title == "" {
			continue
		}

		job := model.JobPost{
			ID:          "getonboard-" + raw.ID,
			Title:       title,
			CompanyName: strings.TrimSpace(attrs.Company),
			CompanyLogo: strings.TrimSpace(attrs.Logo),
			JobURL:      strings.TrimSpace(raw.Links.PublicURL),
			Description: strings.TrimSpace(attrs.Description),
			IsRemote:    attrs.Remote,
			Site:        string(s.SiteName()),
		}

		// Location — location_cities may be []string or {"data":[...]} object
		cityStr := extractCityNames(attrs.LocationCities)
		countryStr := strings.Join(attrs.Countries, ", ")
		if cityStr != "" || countryStr != "" {
			job.Location = model.Location{
				City:    cityStr,
				Country: countryStr,
			}
		}

		// Compensation
		if attrs.MinSalary != nil || attrs.MaxSalary != nil {
			job.Compensation = &model.Compensation{
				Interval:  model.IntervalYearly,
				MinAmount: attrs.MinSalary,
				MaxAmount: attrs.MaxSalary,
				Currency:  "USD",
			}
		}

		// Seniority — may be a string or an object like {"name":"Senior"}
		if seniorityStr := extractSeniority(attrs.Seniority); seniorityStr != "" {
			job.Seniority = seniorityStr
		}

		// DatePosted (Unix timestamp in seconds)
		if attrs.PublishedAt != nil {
			t := time.Unix(*attrs.PublishedAt, 0)
			job.DatePosted = &t
		}

		out = append(out, job)
	}

	util.Debug("scraper_done", map[string]any{
		"site": s.SiteName(),
		"jobs": len(out),
	})

	if !util.HasMeaningfulJobs(out) {
		return nil, fmt.Errorf("getonboard: no parseable jobs")
	}
	return out, nil
}

// extractCityNames gets city names from the location_cities field which may be
// either a []string or a JSON-API object like {"data":[{"id":...,"type":"location_city"}]}.
func extractCityNames(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	// Try []string first
	var strs []string
	if err := json.Unmarshal(raw, &strs); err == nil {
		return strings.Join(strs, ", ")
	}
	// Try object with data array
	var obj struct {
		Data []struct {
			ID   any    `json:"id"`
			Type string `json:"type"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &obj); err == nil {
		// JSON-API relationship — no city names in the data, just references
		_ = obj
	}
	return ""
}

// extractSeniority handles seniority being either a plain string or an object like {"name":"Senior"}.
func extractSeniority(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	// Try plain string first
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return strings.TrimSpace(s)
	}
	// Try object with name field
	var obj struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(raw, &obj); err == nil {
		return strings.TrimSpace(obj.Name)
	}
	return ""
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
