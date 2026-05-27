package jobscore

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
	defaultAPIURL = "https://careers.jobscore.com/jobs"
)

// Location represents a location from JobScore.
type Location struct {
	City    *string `json:"city"`
	State   *string `json:"state"`
	Country *string `json:"country"`
}

// Job represents a single job from JobScore.
type Job struct {
	ID          interface{} `json:"id"`
	Title       *string     `json:"title"`
	DetailURL   *string     `json:"detail_url"`
	Description *string     `json:"description"`
	Department  *string     `json:"department"`
	Location    *Location   `json:"location"`
	CreatedAt   *string     `json:"created_at"`
}

// Scraper for JobScore.
type Scraper struct {
	Client *http.Client
	apiURL string
}

// New creates a new JobScore scraper.
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
func (s *Scraper) SiteName() model.Site { return model.SiteJobScore }

// Scrape fetches jobs from JobScore.
func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{"site": s.SiteName(), "results_wanted": input.ResultsWanted, "search_term": input.SearchTerm})

	seeds, src := ats.ResolveSeeds(input.SearchTerm, "SCRAPPY_JOBSCORE_SEEDS")
	if len(seeds) == 0 {
		return nil, fmt.Errorf("jobscore no seeds: set SCRAPPY_JOBSCORE_SEEDS or pass a company name in --search")
	}
	util.Debug("jobscore_seeds", map[string]any{"seeds": seeds, "src": src})

	var out []model.JobPost
	seen := map[string]struct{}{}

	for _, seed := range seeds {
		if input.ResultsWanted > 0 && len(out) >= input.ResultsWanted {
			break
		}
		jobs, err := s.fetchJobs(ctx, input, seed)
		if err != nil {
			util.Warn("jobscore_seed_fail", map[string]any{"seed": seed, "err": err.Error()})
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
		return nil, fmt.Errorf("jobscore no parseable jobs")
	}
	return out, nil
}

func (s *Scraper) fetchJobs(ctx context.Context, input model.ScraperInput, seed string) ([]model.JobPost, error) {
	resultsWanted := input.ResultsWanted
	if resultsWanted <= 0 {
		resultsWanted = 100
	}

	url := fmt.Sprintf("%s/%s/feed.json?sort=date&limit=%d", s.apiURL, seed, resultsWanted)

	var jobs []Job
	if err := ats.FetchJSON(ctx, s.Client, url, &jobs); err != nil {
		return nil, fmt.Errorf("jobscore fetch: %w", err)
	}

	out := make([]model.JobPost, 0, len(jobs))
	searchTerm := input.SearchTerm

	for _, job := range jobs {
		if job.Title == nil || *job.Title == "" {
			continue
		}

		// Filter by searchTerm (case-insensitive on title and description)
		if searchTerm != "" {
			title := toLower(*job.Title)
			desc := ""
			if job.Description != nil {
				desc = toLower(*job.Description)
			}
			term := toLower(searchTerm)
			if !kontext(title, term) && !kontext(desc, term) {
				continue
			}
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

	// Remote detection
	isRemote := false
	if job.Location != nil {
		if job.Location.City != nil {
			isRemote = caseInsensitiveContains(*job.Location.City, "remote")
		}
		if !isRemote && job.Location.State != nil {
			isRemote = caseInsensitiveContains(*job.Location.State, "remote")
		}
	}

	// Job URL
	jobURL := ""
	if job.DetailURL != nil && *job.DetailURL != "" {
		jobURL = *job.DetailURL
	}
	if jobURL == "" {
		jobURL = fmt.Sprintf("https://careers.jobscore.com/jobs/%s/%s", seed, jobID)
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

	return model.JobPost{
		ID:          ats.BuildID("js", seed, jobID),
		Title:       title,
		CompanyName: seed,
		JobURL:      jobURL,
		Location:    location,
		IsRemote:    isRemote,
		Description: description,
		DatePosted:  datePosted,
		Site:        string(s.SiteName()),
		Department:  department,
	}
}

func toLower(s string) string {
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

func kontext(s, substr string) bool {
	if len(s) < len(substr) || len(substr) == 0 {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func caseInsensitiveContains(s, substr string) bool {
	return kontext(toLower(s), toLower(substr))
}
