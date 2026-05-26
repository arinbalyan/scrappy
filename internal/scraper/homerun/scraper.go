package homerun

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
	defaultAPIURL = "https://app.homerun.co/api/v2/jobs"
)

// Job represents a single job from the Homerun API.
type Job struct {
	ID              interface{} `json:"id"`               // number | string
	Title           *string     `json:"title"`
	Description     *string     `json:"description"`
	Location        *string     `json:"location"`
	Department      *string     `json:"department"`
	EmploymentType  *string     `json:"employment_type"`
	ApplicationURL  *string     `json:"application_url"`
	Slug            *string     `json:"slug"`
	CreatedAt       *string     `json:"created_at"`
	UpdatedAt       *string     `json:"updated_at"`
	Status          *string     `json:"status"`
}

// Response wraps the Homerun API response.
type Response struct {
	Data []Job `json:"data"`
}

// Scraper for Homerun.
type Scraper struct {
	Client *http.Client
	apiURL string
}

// New creates a new Homerun scraper.
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
func (s *Scraper) SiteName() model.Site { return model.SiteHomerun }

// Scrape fetches jobs from Homerun.
func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{"site": s.SiteName(), "results_wanted": input.ResultsWanted, "search_term": input.SearchTerm})

	seeds, src := ats.ResolveSeeds(input.SearchTerm, "SCRAPPY_HOMERUN_SEEDS")
	if len(seeds) == 0 {
		return nil, fmt.Errorf("homerun no seeds: set SCRAPPY_HOMERUN_SEEDS or pass a company name in --search")
	}
	util.Debug("homerun_seeds", map[string]any{"seeds": seeds, "src": src})

	var out []model.JobPost
	seen := map[string]struct{}{}

	for _, seed := range seeds {
		if input.ResultsWanted > 0 && len(out) >= input.ResultsWanted {
			break
		}
		jobs, err := s.fetchJobs(ctx, input, seed)
		if err != nil {
			util.Warn("homerun_seed_fail", map[string]any{"seed": seed, "err": err.Error()})
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
		return nil, fmt.Errorf("homerun no parseable jobs")
	}
	return out, nil
}

func (s *Scraper) fetchJobs(ctx context.Context, input model.ScraperInput, seed string) ([]model.JobPost, error) {
	resultsWanted := input.ResultsWanted
	if resultsWanted <= 0 {
		resultsWanted = 100
	}
	url := fmt.Sprintf("%s?page=1&perPage=%d", s.apiURL, resultsWanted)

	util.Debug("homerun_fetch_url", map[string]any{"url": url})
	var resp Response
	if err := ats.FetchJSON(ctx, s.Client, url, &resp); err != nil {
		util.Warn("homerun_fetch_err", map[string]any{"err": err.Error()})
		return nil, fmt.Errorf("homerun fetch: %w", err)
	}
	util.Debug("homerun_fetch_data_len", map[string]any{"count": len(resp.Data)})

	out := make([]model.JobPost, 0, len(resp.Data))
	searchTerm := input.SearchTerm

	for _, job := range resp.Data {
		if job.Title == nil || *job.Title == "" {
			continue
		}

		// Filter by searchTerm if provided
		if searchTerm != "" && !caseInsensitiveContains(*job.Title, searchTerm) {
			continue
		}

		jp := s.mapJob(job, seed)
		util.Debug("homerun_mapped_job", map[string]any{"title": jp.Title, "url": jp.JobURL, "company": jp.CompanyName})
		out = append(out, jp)
	}
	util.Debug("homerun_fetched", map[string]any{"count": len(out)})
	return out, nil
}

func (s *Scraper) mapJob(job Job, seed string) model.JobPost {
	jobID := fmt.Sprintf("%v", job.ID)
	title := ""
	if job.Title != nil {
		title = *job.Title
	}

	// Description
	description := ""
	if job.Description != nil {
		description = util.StripHTML(*job.Description)
	}

	// Location
	location := model.Location{}
	locationStr := ""
	if job.Location != nil {
		locationStr = *job.Location
		location.City = locationStr
	}

	// Remote detection
	isRemote := false
	if locationStr != "" {
		isRemote = caseInsensitiveContains(locationStr, "remote")
	}

	// Job URL
	jobURL := ""
	if job.ApplicationURL != nil && *job.ApplicationURL != "" {
		jobURL = *job.ApplicationURL
	}
	if jobURL == "" && job.Slug != nil && *job.Slug != "" {
		jobURL = fmt.Sprintf("https://app.homerun.co/%s/%s", seed, *job.Slug)
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
	}

	// Employment type
	employmentType := ""
	if job.EmploymentType != nil {
		employmentType = *job.EmploymentType
	}

	_ = employmentType // used for job type

	jp := model.JobPost{
		ID:          ats.BuildID("hm", seed, jobID),
		Title:       title,
		CompanyName: seed,
		JobURL:      jobURL,
		Location:    location,
		IsRemote:    isRemote,
		Description: description,
		DatePosted:  datePosted,
		Site:        string(s.SiteName()),
		Department:  department,
		JobType:     employmentType,
	}
	return jp
}

func caseInsensitiveContains(s, substr string) bool {
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			sc := s[i+j]
			tc := substr[j]
			if sc >= 'A' && sc <= 'Z' {
				sc += 32
			}
			if tc >= 'A' && tc <= 'Z' {
				tc += 32
			}
			if sc != tc {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
