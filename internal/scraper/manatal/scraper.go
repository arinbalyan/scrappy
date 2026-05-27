package manatal

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/scraper/ats"
	"github.com/arinbalyan/scrappy/internal/util"
)

const (
	defaultAPIURL = "https://api.manatal.com/open/v1/career-page"
)

// Location represents a location from Manatal.
type Location struct {
	City    *string `json:"city"`
	State   *string `json:"state"`
	Country *string `json:"country"`
}

// Job represents a single job from Manatal.
type Job struct {
	ID           int       `json:"id"`
	PositionName string    `json:"position_name"`
	Description  string    `json:"description"`
	Requirement  *string   `json:"requirement"`
	Department   *string   `json:"department"`
	Location     *Location `json:"location"`
	EmploymentType *string `json:"employment_type"`
	SalaryMin    *float64  `json:"salary_min"`
	SalaryMax    *float64  `json:"salary_max"`
	SalaryCurrency *string `json:"salary_currency"`
	CreatedAt    string    `json:"created_at"`
	UpdatedAt    string    `json:"updated_at"`
	ApplyURL     *string   `json:"apply_url"`
	CareerPageURL *string  `json:"career_page_url"`
}

// Response wraps the Manatal API response.
type Response struct {
	Count   int    `json:"count"`
	Next    *string `json:"next"`
	Results []Job  `json:"results"`
}

// Scraper for Manatal.
type Scraper struct {
	Client *http.Client
	apiURL string
}

// New creates a new Manatal scraper.
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
func (s *Scraper) SiteName() model.Site { return model.SiteManatal }

// Scrape fetches jobs from Manatal.
func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{"site": s.SiteName(), "results_wanted": input.ResultsWanted, "search_term": input.SearchTerm})

	seeds, src := ats.ResolveSeeds(input.SearchTerm, "SCRAPPY_MANATAL_SEEDS")
	if len(seeds) == 0 {
		return nil, fmt.Errorf("manatal no seeds: set SCRAPPY_MANATAL_SEEDS or pass a company name in --search")
	}
	util.Debug("manatal_seeds", map[string]any{"seeds": seeds, "src": src})

	var out []model.JobPost
	seen := map[string]struct{}{}

	for _, seed := range seeds {
		if input.ResultsWanted > 0 && len(out) >= input.ResultsWanted {
			break
		}
		jobs, err := s.fetchJobs(ctx, input, seed)
		if err != nil {
			util.Warn("manatal_seed_fail", map[string]any{"seed": seed, "err": err.Error()})
			continue
		}
		for _, jp := range jobs {
			if _, ok := seen[jp.ID]; ok {
				continue
			}
			seen[jp.ID] = struct{}{}
			out = append(out, jp)
			if input.ResultsWanted > 0 && len(out) >= input.ResultsWanted {
				break
			}
		}
	}

	if !util.HasMeaningfulJobs(out) {
		return nil, fmt.Errorf("manatal no parseable jobs")
	}
	return out, nil
}

func (s *Scraper) fetchJobs(ctx context.Context, input model.ScraperInput, seed string) ([]model.JobPost, error) {
	url := fmt.Sprintf("%s/%s/jobs/", s.apiURL, seed)

	var resp Response
	if err := ats.FetchJSON(ctx, s.Client, url, &resp); err != nil {
		return nil, fmt.Errorf("manatal fetch: %w", err)
	}

	out := make([]model.JobPost, 0, len(resp.Results))
	resultsWanted := input.ResultsWanted
	if resultsWanted <= 0 {
		resultsWanted = 100
	}

	for _, job := range resp.Results {
		if len(out) >= resultsWanted {
			break
		}
		if job.PositionName == "" {
			continue
		}
		jp := s.mapJob(job, seed)
		out = append(out, jp)
	}
	return out, nil
}

func (s *Scraper) mapJob(job Job, seed string) model.JobPost {
	// Description
	description := ""
	if job.Description != "" {
		description = util.StripHTML(job.Description)
	}

	// Location
	location := model.Location{}
	if job.Location != nil {
		if job.Location.City != nil {
			location.City = *job.Location.City
		}
		if job.Location.State != nil {
			location.State = *job.Location.State
		}
		if job.Location.Country != nil {
			location.Country = *job.Location.Country
		}
	}

	// Compensation
	var compensation *model.Compensation
	if job.SalaryMin != nil || job.SalaryMax != nil {
		currency := "USD"
		if job.SalaryCurrency != nil && *job.SalaryCurrency != "" {
			currency = *job.SalaryCurrency
		}
		compensation = &model.Compensation{
			Interval:  model.IntervalYearly,
			MinAmount: job.SalaryMin,
			MaxAmount: job.SalaryMax,
			Currency:  currency,
		}
	}

	// Job URL
	jobURL := ""
	if job.ApplyURL != nil && *job.ApplyURL != "" {
		jobURL = *job.ApplyURL
	} else if job.CareerPageURL != nil && *job.CareerPageURL != "" {
		jobURL = *job.CareerPageURL
	}
	if jobURL == "" {
		jobURL = fmt.Sprintf("%s/%s/jobs/%d/", s.apiURL, seed, job.ID)
	}

	// Date posted
	var datePosted *time.Time
	if job.CreatedAt != "" {
		datePosted = util.ParseDatePosted(job.CreatedAt)
	}

	// Department
	department := ""
	if job.Department != nil {
		department = *job.Department
	}

	return model.JobPost{
		ID:           ats.BuildID("manatal", seed, fmt.Sprintf("%d", job.ID)),
		Title:        job.PositionName,
		CompanyName:  seed,
		JobURL:       jobURL,
		Location:     location,
		Description:  description,
		DatePosted:   datePosted,
		Site:         string(s.SiteName()),
		Department:   department,
		JobType:      safeStringManatal(job.EmploymentType),
		Compensation: compensation,
	}
}

func safeStringManatal(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
