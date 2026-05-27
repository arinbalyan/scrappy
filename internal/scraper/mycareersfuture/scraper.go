package mycareersfuture

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

const baseURL = "https://api.mycareersfuture.gov.sg/v2/jobs"

// Internal JSON response types (mirrors the MyCareersFuture API).
type (
	mycareersfutureSalary struct {
		Minimum  float64 `json:"minimum"`
		Maximum  float64 `json:"maximum"`
		Currency string  `json:"currency"`
	}

	mycareersfutureCompany struct {
		Name string `json:"name"`
	}

	mycareersfutureLocation struct {
		Name string `json:"name"`
	}

	mycareersfutureJob struct {
		UUID        string                  `json:"uuid"`
		Title       string                  `json:"title"`
		Description string                  `json:"description"`
		Company     *mycareersfutureCompany `json:"company"`
		Salary      *mycareersfutureSalary  `json:"salary"`
		Location    *mycareersfutureLocation `json:"location"`
		PostedDate  string                  `json:"postedDate"`
	}

	mycareersfutureResponse struct {
		Results []mycareersfutureJob `json:"results"`
	}
)

// Scraper scrapes MyCareersFuture (mycareersfuture.gov.sg) job listings via their JSON API.
type Scraper struct {
	client    *http.Client
	searchURL string
}

// New creates a new MyCareersFuture scraper.
func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Retries: 3, Timeout: 25 * time.Second})
	}
	return &Scraper{client: client, searchURL: baseURL}
}

// NewWithURLs creates a scraper with an overridable endpoint (used in tests).
func NewWithURLs(client *http.Client, endpoint string) *Scraper {
	s := New(client)
	if strings.TrimSpace(endpoint) != "" {
		s.searchURL = strings.TrimSpace(endpoint)
	}
	return s
}

// SiteName returns the site identifier.
func (s *Scraper) SiteName() model.Site { return model.SiteMyCareersFuture }

// Scrape fetches job listings from MyCareersFuture with rate limiting (3 req/s).
func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	wanted := input.ResultsWanted
	if wanted <= 0 {
		wanted = 25
	}

	util.Debug("scraper_mycareersfuture_start", map[string]any{
		"search_term":    input.SearchTerm,
		"location":       input.Location,
		"results_wanted": wanted,
	})

	body, err := s.fetchPage(ctx, input.SearchTerm, wanted)
	if err != nil {
		return nil, fmt.Errorf("mycareersfuture fetch: %w", err)
	}

	jobs, err := parseJobs(body)
	if err != nil {
		return nil, fmt.Errorf("mycareersfuture parse: %w", err)
	}

	if !util.HasMeaningfulJobs(jobs) {
		return nil, fmt.Errorf("mycareersfuture: no parseable jobs")
	}

	if len(jobs) > wanted {
		jobs = jobs[:wanted]
	}

	util.Debug("scraper_mycareersfuture_done", map[string]any{
		"jobs_found": len(jobs),
	})

	return jobs, nil
}

func (s *Scraper) fetchPage(ctx context.Context, searchTerm string, limit int) ([]byte, error) {
	u, err := url.Parse(s.searchURL)
	if err != nil {
		return nil, fmt.Errorf("parse url: %w", err)
	}

	q := u.Query()
	q.Set("limit", fmt.Sprintf("%d", limit))
	if v := strings.TrimSpace(searchTerm); v != "" {
		q.Set("keyword", v)
	}
	u.RawQuery = q.Encode()

	// Rate limit: 200-500ms jittered delay
	if err := util.JitterSleep(ctx, 200*time.Millisecond, 300*time.Millisecond); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Origin", "https://www.mycareersfuture.gov.sg")
	req.Header.Set("Referer", "https://www.mycareersfuture.gov.sg/")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("mycareersfuture status %d — try using --proxy with a residential proxy", resp.StatusCode)
	}

	body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	return body, nil
}

// parseJobs unmarshals the JSON API response and maps results to JobPost.
func parseJobs(body []byte) ([]model.JobPost, error) {
	var apiResp mycareersfutureResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("json decode: %w", err)
	}

	if len(apiResp.Results) == 0 {
		return nil, nil
	}

	out := make([]model.JobPost, 0, len(apiResp.Results))
	for _, raw := range apiResp.Results {
		if raw.UUID == "" {
			continue
		}
		title := strings.TrimSpace(raw.Title)
		if title == "" {
			continue
		}

		jobURL := fmt.Sprintf("https://www.mycareersfuture.gov.sg/job/%s", raw.UUID)

		loc := model.Location{Country: "Singapore"}
		if raw.Location != nil && strings.TrimSpace(raw.Location.Name) != "" {
			loc.City = strings.TrimSpace(raw.Location.Name)
		}

		var comp *model.Compensation
		if raw.Salary != nil && (raw.Salary.Minimum > 0 || raw.Salary.Maximum > 0) {
			minVal := raw.Salary.Minimum
			maxVal := raw.Salary.Maximum
			currency := strings.TrimSpace(raw.Salary.Currency)
			if currency == "" {
				currency = "SGD"
			}
			comp = &model.Compensation{
				Interval: model.IntervalMonthly,
				Currency: currency,
			}
			if minVal > 0 {
				comp.MinAmount = &minVal
			}
			if maxVal > 0 {
				comp.MaxAmount = &maxVal
			}
		}

		var datePosted *time.Time
		if v := strings.TrimSpace(raw.PostedDate); v != "" {
			datePosted = util.ParseDatePosted(v)
		}

		description := strings.TrimSpace(raw.Description)

		companyName := ""
		if raw.Company != nil && strings.TrimSpace(raw.Company.Name) != "" {
			companyName = strings.TrimSpace(raw.Company.Name)
		}

		job := model.JobPost{
			ID:          "mcf-" + raw.UUID,
			Title:       title,
			CompanyName: companyName,
			JobURL:      jobURL,
			Location:    loc,
			Description: description,
			Compensation: comp,
			DatePosted:  datePosted,
			IsRemote:    false,
		}
		out = append(out, job)
	}

	return out, nil
}
