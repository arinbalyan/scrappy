package freelancercom

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

const defaultAPI = "https://www.freelancer.com/api/projects/0.1/projects/active"

// Scraper fetches projects from the Freelancer.com API.
type Scraper struct {
	client *http.Client
	apiURL string
}

// New creates a new Freelancer.com scraper. If client is nil a default one is used.
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
func (s *Scraper) SiteName() model.Site { return model.SiteFreelancerCom }

// --- API response types ---

type freelancerResponse struct {
	Status string                `json:"status"`
	Result *freelancerResult     `json:"result,omitempty"`
}

type freelancerResult struct {
	Projects   []freelancerProject `json:"projects"`
	TotalCount int                 `json:"total_count"`
}

type freelancerProject struct {
	ID       int                   `json:"id"`
	Title    string                `json:"title"`
	Description string             `json:"description"`
	SeoURL   string                `json:"seo_url"`
	Type     string                `json:"type"`
	Currency *freelancerCurrency   `json:"currency,omitempty"`
	Budget   *freelancerBudget     `json:"budget,omitempty"`
	TimeSubmitted *int64           `json:"time_submitted"`
	Location *freelancerLocation   `json:"location,omitempty"`
	Owner    *freelancerOwner      `json:"owner,omitempty"`
}

type freelancerCurrency struct {
	Code string `json:"code"`
	Sign string `json:"sign"`
}

type freelancerBudget struct {
	Minimum *float64 `json:"minimum"`
	Maximum *float64 `json:"maximum"`
}

type freelancerLocation struct {
	City    string                `json:"city"`
	Country *freelancerCountry    `json:"country,omitempty"`
}

type freelancerCountry struct {
	Name string `json:"name"`
}

type freelancerOwner struct {
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
}

// Scrape fetches projects from Freelancer.com.
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
	if wanted > 50 {
		wanted = 50
	}

	u, _ := url.Parse(s.apiURL)
	q := url.Values{}
	q.Set("compact", "true")
	q.Set("limit", strconv.Itoa(wanted))
	q.Set("full_description", "true")
	if input.SearchTerm != "" {
		q.Set("query", input.SearchTerm)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("freelancercom: build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("freelancercom: request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("freelancercom: status %d", resp.StatusCode)
	}

	body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("freelancercom: read: %w", err)
	}

	var apiResp freelancerResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("freelancercom: decode: %w", err)
	}

	if apiResp.Result == nil || len(apiResp.Result.Projects) == 0 {
		return nil, fmt.Errorf("freelancercom: no projects returned")
	}

	out := make([]model.JobPost, 0, wanted)
	for _, p := range apiResp.Result.Projects {
		if len(out) >= wanted {
			break
		}

		title := strings.TrimSpace(p.Title)
		if title == "" {
			continue
		}

		job := model.JobPost{
			ID:          fmt.Sprintf("freelancercom-%d", p.ID),
			Title:       title,
			Description: strings.TrimSpace(p.Description),
			IsRemote:    true,
			Site:        string(s.SiteName()),
		}

		// Job URL
		if p.SeoURL != "" {
			job.JobURL = "https://www.freelancer.com/projects/" + p.SeoURL
		} else {
			job.JobURL = fmt.Sprintf("https://www.freelancer.com/projects/%d", p.ID)
		}

		// Company / Owner name
		if p.Owner != nil && strings.TrimSpace(p.Owner.DisplayName) != "" {
			job.CompanyName = strings.TrimSpace(p.Owner.DisplayName)
		}

		// Location
		if p.Location != nil {
			city := strings.TrimSpace(p.Location.City)
			country := ""
			if p.Location.Country != nil {
				country = strings.TrimSpace(p.Location.Country.Name)
			}
			if city != "" || country != "" {
				job.Location = model.Location{City: city, Country: country}
			}
		}

		// Compensation
		if p.Budget != nil && (p.Budget.Minimum != nil || p.Budget.Maximum != nil) {
			currency := "USD"
			if p.Currency != nil && strings.TrimSpace(p.Currency.Code) != "" {
				currency = strings.TrimSpace(p.Currency.Code)
			}
			interval := model.IntervalHourly
			if p.Type != "hourly" {
				interval = model.IntervalYearly
			}
			job.Compensation = &model.Compensation{
				Interval:  interval,
				MinAmount: p.Budget.Minimum,
				MaxAmount: p.Budget.Maximum,
				Currency:  currency,
			}
		}

		// DatePosted (Unix timestamp)
		if p.TimeSubmitted != nil {
			t := time.Unix(*p.TimeSubmitted, 0)
			job.DatePosted = &t
		}

		out = append(out, job)
	}

	util.Debug("scraper_done", map[string]any{
		"site": s.SiteName(),
		"jobs": len(out),
	})

	if !util.HasMeaningfulJobs(out) {
		return nil, fmt.Errorf("freelancercom: no parseable jobs")
	}
	return out, nil
}
