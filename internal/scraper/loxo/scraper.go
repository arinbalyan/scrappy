package loxo

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/util"
)

const (
	defaultAPIURL = "https://app.loxo.co/api"
)

// JobLocation represents a location from Loxo.
type JobLocation struct {
	City    *string `json:"city"`
	State   *string `json:"state"`
	Country *string `json:"country"`
}

// Compensation represents salary info from Loxo.
type Compensation struct {
	Min      *float64 `json:"min"`
	Max      *float64 `json:"max"`
	Currency *string  `json:"currency"`
	Interval *string  `json:"interval"`
}

// Job represents a single job from Loxo.
type Job struct {
	ID             interface{}      `json:"id"`
	Title          *string          `json:"title"`
	Description    *string          `json:"description"`
	Location       json.RawMessage  `json:"location"` // can be string or object
	Department     *string          `json:"department"`
	Type           *string          `json:"type"`
	EmploymentType *string          `json:"employment_type"`
	CreatedAt      *string          `json:"created_at"`
	URL            *string          `json:"url"`
	ApplyURL       *string          `json:"apply_url"`
	Remote         *bool            `json:"remote"`
	Salary         *Compensation    `json:"salary"`
	Compensation   *Compensation    `json:"compensation"`
	Category       *string          `json:"category"`
	CompanyName    *string          `json:"company_name"`
}

// Scraper for Loxo.
type Scraper struct {
	Client *http.Client
	apiURL string
}

// New creates a new Loxo scraper.
func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Retries: 2, Timeout: 15 * time.Second})
	}
	return &Scraper{Client: client, apiURL: defaultAPIURL}
}

// NewWithAPIURL creates a scraper with a custom API URL.
func NewWithAPIURL(client *http.Client, apiURL string) *Scraper {
	s := New(client)
	if apiURL != "" {
		s.apiURL = apiURL
	}
	return s
}

// SiteName returns the site identifier.
func (s *Scraper) SiteName() model.Site { return model.SiteLoxo }

// Scrape fetches jobs from Loxo.
func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{"site": s.SiteName(), "results_wanted": input.ResultsWanted, "search_term": input.SearchTerm})

	// For Loxo, the search term IS the company slug (no env var support).
	// The URL is: https://app.loxo.co/api/{slug}/jobs
	seed := input.SearchTerm
	if seed == "" {
		return nil, fmt.Errorf("loxo requires a company slug as --search term")
	}
	util.Debug("loxo_seed", map[string]any{"seed": seed})

	jobs, err := s.fetchJobs(ctx, input, seed)
	if err != nil {
		return nil, fmt.Errorf("loxo scrape: %w", err)
	}

	if !util.HasMeaningfulJobs(jobs) {
		return nil, fmt.Errorf("loxo no parseable jobs")
	}
	return jobs, nil
}

func (s *Scraper) fetchJobs(ctx context.Context, input model.ScraperInput, seed string) ([]model.JobPost, error) {
	url := fmt.Sprintf("%s/%s/jobs", s.apiURL, seed)

	var raw json.RawMessage
	if err := fetchJSON(ctx, s.Client, url, &raw); err != nil {
		return nil, fmt.Errorf("loxo fetch: %w", err)
	}

	// The API may return an array directly or an object with a jobs key
	var jobs []Job
	if err := json.Unmarshal(raw, &jobs); err == nil {
		// It's a direct array
	} else {
		// Try as object with jobs key
		var wrapper struct {
			Jobs []Job `json:"jobs"`
		}
		if err2 := json.Unmarshal(raw, &wrapper); err2 != nil {
			return nil, fmt.Errorf("loxo decode: %w", err)
		}
		jobs = wrapper.Jobs
	}

	out := make([]model.JobPost, 0, len(jobs))
	resultsWanted := input.ResultsWanted
	if resultsWanted <= 0 {
		resultsWanted = 100
	}

	for _, job := range jobs {
		if len(out) >= resultsWanted {
			break
		}
		if job.Title == nil || *job.Title == "" {
			continue
		}
		jp := s.mapJob(job, seed)
		out = append(out, jp)
	}
	return out, nil
}

func (s *Scraper) mapJob(job Job, seed string) model.JobPost {
	title := ""
	if job.Title != nil {
		title = *job.Title
	}

	jobID := fmt.Sprintf("%v", job.ID)

	// Description
	description := ""
	if job.Description != nil {
		description = util.StripHTML(*job.Description)
	}

	// Location — can be string or structured object
	location := model.Location{}
	isRemote := false

	if job.Location != nil {
		// Try string first
		var locStr string
		if err := json.Unmarshal(job.Location, &locStr); err == nil {
			location.City = locStr
			isRemote = stringsContains(stringsToLower(locStr), "remote")
		} else {
			// Try object
			var locObj JobLocation
			if err := json.Unmarshal(job.Location, &locObj); err == nil {
				if locObj.City != nil {
					location.City = *locObj.City
				}
				if locObj.State != nil {
					location.State = *locObj.State
				}
				if locObj.Country != nil {
					location.Country = *locObj.Country
				}
			}
		}
	}

	// Remote flag from API
	if !isRemote && job.Remote != nil && *job.Remote {
		isRemote = true
	}

	// Also check title
	if !isRemote && stringsContains(stringsToLower(title), "remote") {
		isRemote = true
	}

	// Compensation
	var compensation *model.Compensation
	if job.Salary != nil {
		compensation = parseCompensation(job.Salary)
	} else if job.Compensation != nil {
		compensation = parseCompensation(job.Compensation)
	}

	// Job URL
	jobURL := ""
	if job.URL != nil && *job.URL != "" {
		jobURL = *job.URL
	} else if job.ApplyURL != nil && *job.ApplyURL != "" {
		jobURL = *job.ApplyURL
	}
	if jobURL == "" {
		jobURL = fmt.Sprintf("%s/%s/jobs/%s", s.apiURL, seed, jobID)
	}

	// Date posted
	var datePosted *time.Time
	if job.CreatedAt != nil && *job.CreatedAt != "" {
		datePosted = util.ParseDatePosted(*job.CreatedAt)
	}

	// Department
	department := ""
	if job.Department != nil {
		department = *job.Department
	} else if job.Category != nil {
		department = *job.Category
	}

	// Employment type
	employmentType := ""
	if job.EmploymentType != nil {
		employmentType = *job.EmploymentType
	} else if job.Type != nil {
		employmentType = *job.Type
	}

	// Company name
	companyName := seed
	if job.CompanyName != nil && *job.CompanyName != "" {
		companyName = *job.CompanyName
	}

	jp := model.JobPost{
		ID:           atsBuildID("loxo", seed, jobID),
		Title:        title,
		CompanyName:  companyName,
		JobURL:       jobURL,
		Location:     location,
		IsRemote:     isRemote,
		Description:  description,
		DatePosted:   datePosted,
		Site:         string(s.SiteName()),
		Department:   department,
		JobType:      employmentType,
		Compensation: compensation,
	}

	return jp
}

func parseCompensation(c *Compensation) *model.Compensation {
	if c.Min == nil && c.Max == nil {
		return nil
	}

	interval := model.IntervalYearly
	if c.Interval != nil {
		upper := stringsToLower(*c.Interval)
		if stringsContains(upper, "month") {
			interval = model.IntervalMonthly
		} else if stringsContains(upper, "week") {
			interval = model.IntervalWeekly
		} else if stringsContains(upper, "day") {
			interval = model.IntervalDaily
		} else if stringsContains(upper, "hour") {
			interval = model.IntervalHourly
		}
	}

	currency := "USD"
	if c.Currency != nil && *c.Currency != "" {
		currency = *c.Currency
	}

	return &model.Compensation{
		Interval: interval,
		MinAmount: c.Min,
		MaxAmount: c.Max,
		Currency:  currency,
	}
}

func atsBuildID(prefix, slug, jobID string) string {
	raw := slug + "-" + jobID
	return prefix + "-" + util.HashID(raw)
}

func fetchJSON(ctx context.Context, client *http.Client, url string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("status %d", resp.StatusCode)
	}

	body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return fmt.Errorf("read: %w", err)
	}

	if err := json.Unmarshal(body, target); err != nil {
		return fmt.Errorf("decode: %w", err)
	}
	return nil
}

func stringsToLower(s string) string {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 32
		}
		b[i] = c
	}
	return string(b)
}

func stringsContains(s, substr string) bool {
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
