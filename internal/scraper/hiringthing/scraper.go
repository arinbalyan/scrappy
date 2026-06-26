package hiringthing

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
	defaultAPIURL = "https://api.hiringthing.com/api/v1/jobs"
)

// Job represents a single job from the HiringThing API.
type Job struct {
	ID          interface{} `json:"id"`          // number | string
	Title       string      `json:"title"`
	Description *string     `json:"description"`
	Location    *string     `json:"location"`
	Department  *string     `json:"department"`
	Type        *string     `json:"type"`
	CreatedAt   *string     `json:"created_at"`
	URL         *string     `json:"url"`
	CompanyName *string     `json:"company_name"`
	Status      *string     `json:"status"`
	Salary      *string     `json:"salary"`
	Experience  *string     `json:"experience"`
}

// Response wraps the HiringThing API response.
type Response struct {
	Jobs []Job `json:"jobs"`
}

// Scraper for HiringThing.
type Scraper struct {
	Client  *http.Client
	apiURL  string
}

// New creates a new HiringThing scraper.
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
func (s *Scraper) SiteName() model.Site { return model.SiteHiringThing }

// Scrape fetches jobs from HiringThing.
func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{"site": s.SiteName(), "results_wanted": input.ResultsWanted, "search_term": input.SearchTerm})

	seeds, src := ats.ResolveSeeds(input.SearchTerm, "SCRAPPY_HIRINGTHING_SEEDS")
	if len(seeds) == 0 {
		return nil, fmt.Errorf("hiringthing no seeds: set SCRAPPY_HIRINGTHING_SEEDS or pass a company name in --search")
	}
	util.Debug("hiringthing_seeds", map[string]any{"seeds": seeds, "src": src})

	wanted := input.ResultsWanted
	if wanted <= 0 {
		wanted = 25
	}

	fetchFn := func(ctx context.Context, slug string) ([]model.JobPost, error) {
		jobs, err := s.fetchJobs(ctx, input, slug)
		if err != nil {
			util.Warn("hiringthing_seed_fail", map[string]any{"seed": slug, "err": err.Error()})
			return nil, err
		}
		return jobs, nil
	}

	results := ats.ProcessSeeds(ctx, seeds, 3, wanted, fetchFn)
	if !util.HasMeaningfulJobs(results) {
		return nil, fmt.Errorf("hiringthing no parseable jobs")
	}
	return results, nil
}

func (s *Scraper) fetchJobs(ctx context.Context, input model.ScraperInput, seed string) ([]model.JobPost, error) {
	var resp Response
	if err := ats.FetchJSON(ctx, s.Client, s.apiURL, &resp); err != nil {
		return nil, fmt.Errorf("hiringthing fetch: %w", err)
	}

	out := make([]model.JobPost, 0, len(resp.Jobs))
	for _, job := range resp.Jobs {
		if job.Title == "" {
			continue
		}
		jp := s.mapJob(job, seed)
		out = append(out, jp)
	}
	return out, nil
}

func (s *Scraper) mapJob(job Job, seed string) model.JobPost {
	jobID := fmt.Sprintf("%v", job.ID)

	// Description — API returns HTML
	description := ""
	if job.Description != nil {
		description = util.StripHTML(*job.Description)
	}

	// Location
	location := model.Location{}
	if job.Location != nil {
		location.City = *job.Location
	}

	// Job URL
	jobURL := ""
	if job.URL != nil && *job.URL != "" {
		jobURL = *job.URL
	} else {
		jobURL = fmt.Sprintf("https://api.hiringthing.com/jobs/%s", jobID)
	}

	// Date posted
	var datePosted *time.Time
	if job.CreatedAt != nil && *job.CreatedAt != "" {
		datePosted = util.ParseDatePosted(*job.CreatedAt)
	}

	// Company name
	companyName := seed
	if job.CompanyName != nil && *job.CompanyName != "" {
		companyName = *job.CompanyName
	}

	jp := model.JobPost{
		ID:          ats.BuildID("ht", seed, jobID),
		Title:       job.Title,
		CompanyName: companyName,
		JobURL:      jobURL,
		Location:    location,
		Description: description,
		DatePosted:  datePosted,
		Site:        string(s.SiteName()),
	}

	return jp
}
