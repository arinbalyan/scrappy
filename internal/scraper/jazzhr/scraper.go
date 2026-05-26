package jazzhr

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/scraper/ats"
	"github.com/arinbalyan/scrappy/internal/util"
)

const (
	apiBaseURL = "https://api.resumatorapi.com/v1/jobs/status/open"
)

// APIJob represents a job from the JazzHR REST API.
type APIJob struct {
	ID               *string `json:"id"`
	Title            *string `json:"title"`
	City             *string `json:"city"`
	State            *string `json:"state"`
	Zip              *string `json:"zip"`
	Department       *string `json:"department"`
	Description      *string `json:"description"`
	Type             *string `json:"type"`
	OriginalOpenDate *string `json:"original_open_date"`
	BoardCode        *string `json:"board_code"`
}

// Scraper for JazzHR.
type Scraper struct {
	Client *http.Client
	apiURL string
}

// New creates a new JazzHR scraper.
func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Retries: 2, Timeout: 15 * time.Second})
	}
	return &Scraper{Client: client, apiURL: apiBaseURL}
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
func (s *Scraper) SiteName() model.Site { return model.SiteJazzHR }

// Scrape fetches jobs from JazzHR using the authenticated API.
func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{"site": s.SiteName(), "results_wanted": input.ResultsWanted, "search_term": input.SearchTerm})

	seeds, src := ats.ResolveSeeds(input.SearchTerm, "SCRAPPY_JAZZHR_SEEDS")
	if len(seeds) == 0 {
		return nil, fmt.Errorf("jazzhr no seeds: set SCRAPPY_JAZZHR_SEEDS or pass a company name in --search")
	}
	util.Debug("jazzhr_seeds", map[string]any{"seeds": seeds, "src": src})

	// Resolve API key
	apiKey := os.Getenv("JAZZHR_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("jazzhr requires JAZZHR_API_KEY env var")
	}

	var out []model.JobPost
	seen := map[string]struct{}{}

	for _, seed := range seeds {
		if input.ResultsWanted > 0 && len(out) >= input.ResultsWanted {
			break
		}
		jobs, err := s.fetchAPI(ctx, input, apiKey, seed)
		if err != nil {
			util.Warn("jazzhr_seed_fail", map[string]any{"seed": seed, "err": err.Error()})
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
		return nil, fmt.Errorf("jazzhr no parseable jobs")
	}
	return out, nil
}

func (s *Scraper) fetchAPI(ctx context.Context, input model.ScraperInput, apiKey string, seed string) ([]model.JobPost, error) {
	url := fmt.Sprintf("%s?apikey=%s", s.apiURL, apiKey)

	var jobs []APIJob
	if err := ats.FetchJSON(ctx, s.Client, url, &jobs); err != nil {
		return nil, fmt.Errorf("jazzhr api fetch: %w", err)
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

func (s *Scraper) mapJob(job APIJob, seed string) model.JobPost {
	jobID := ""
	if job.ID != nil {
		jobID = *job.ID
	}

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
	if job.City != nil {
		location.City = *job.City
	}
	if job.State != nil {
		location.State = *job.State
	}

	// Job URL
	jobURL := ""
	if job.BoardCode != nil && *job.BoardCode != "" {
		jobURL = fmt.Sprintf("https://%s.applytojob.com/apply/%s", seed, *job.BoardCode)
	} else {
		jobURL = fmt.Sprintf("https://%s.applytojob.com/apply/%s", seed, jobID)
	}

	// Date posted
	var datePosted *time.Time
	if job.OriginalOpenDate != nil && *job.OriginalOpenDate != "" {
		datePosted = util.ParseDatePosted(*job.OriginalOpenDate)
	}

	// Department
	department := ""
	if job.Department != nil {
		department = *job.Department
	}

	// Employment type
	employmentType := ""
	if job.Type != nil {
		employmentType = *job.Type
	}

	return model.JobPost{
		ID:          ats.BuildID("jazzhr", seed, jobID),
		Title:       title,
		CompanyName: seed,
		JobURL:      jobURL,
		Location:    location,
		Description: description,
		DatePosted:  datePosted,
		Site:        string(s.SiteName()),
		Department:  department,
		JobType:     employmentType,
	}
}
