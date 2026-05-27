package jobylon

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
	defaultAPIURL = "https://feed.jobylon.com/feeds"
)

// Company represents the company info from Jobylon.
type Company struct {
	Name *string `json:"name"`
}

// Location represents a location from Jobylon.
type Location struct {
	City    *string `json:"city"`
	Country *string `json:"country"`
}

// URLs holds URLs for a job.
type URLs struct {
	Ad    *string `json:"ad"`
	Apply *string `json:"apply"`
}

// Skill represents a skill from Jobylon.
type Skill struct {
	Label *string `json:"label"`
}

// Job represents a single job from Jobylon.
type Job struct {
	ID            *int        `json:"id"`
	Title         *string     `json:"title"`
	Slug          *string     `json:"slug"`
	Description   *string     `json:"description"`
	Company       *Company    `json:"company"`
	Locations     []Location  `json:"locations"`
	URLs          *URLs       `json:"urls"`
	FromDate      *string     `json:"from_date"`
	ToDate        *string     `json:"to_date"`
	EmploymentType *string    `json:"employment_type"`
	WorkspaceType *string     `json:"workspace_type"`
	Skills        []Skill     `json:"skills"`
	Department    *string     `json:"department"`
	Experience    *string     `json:"experience"`
}

// Scraper for Jobylon.
type Scraper struct {
	Client *http.Client
	apiURL string
}

// New creates a new Jobylon scraper.
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
func (s *Scraper) SiteName() model.Site { return model.SiteJobylon }

// Scrape fetches jobs from Jobylon.
func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{"site": s.SiteName(), "results_wanted": input.ResultsWanted, "search_term": input.SearchTerm})

	seeds, src := ats.ResolveSeeds(input.SearchTerm, "SCRAPPY_JOBYLON_SEEDS")
	if len(seeds) == 0 {
		return nil, fmt.Errorf("jobylon no seeds: set SCRAPPY_JOBYLON_SEEDS or pass a company name in --search")
	}
	util.Debug("jobylon_seeds", map[string]any{"seeds": seeds, "src": src})

	var out []model.JobPost
	seen := map[string]struct{}{}

	for _, seed := range seeds {
		if input.ResultsWanted > 0 && len(out) >= input.ResultsWanted {
			break
		}
		jobs, err := s.fetchJobs(ctx, input, seed)
		if err != nil {
			util.Warn("jobylon_seed_fail", map[string]any{"seed": seed, "err": err.Error()})
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
		return nil, fmt.Errorf("jobylon no parseable jobs")
	}
	return out, nil
}

func (s *Scraper) fetchJobs(ctx context.Context, input model.ScraperInput, seed string) ([]model.JobPost, error) {
	url := fmt.Sprintf("%s/%s/?format=json", s.apiURL, seed)

	var jobs []Job
	if err := ats.FetchJSON(ctx, s.Client, url, &jobs); err != nil {
		return nil, fmt.Errorf("jobylon fetch: %w", err)
	}

	out := make([]model.JobPost, 0, len(jobs))
	searchTerm := input.SearchTerm
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

		// Filter by searchTerm (case-insensitive title match)
		if searchTerm != "" && !caseInsensitiveContains(*job.Title, searchTerm) {
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

	jobID := ""
	if job.ID != nil {
		jobID = fmt.Sprintf("%d", *job.ID)
	}

	// Description
	description := ""
	if job.Description != nil {
		description = util.StripHTML(*job.Description)
	}

	// Location from first location object
	location := model.Location{}
	if len(job.Locations) > 0 {
		loc := job.Locations[0]
		if loc.City != nil {
			location.City = *loc.City
		}
		if loc.Country != nil {
			location.Country = *loc.Country
		}
	}

	// Remote detection
	isRemote := false
	if job.WorkspaceType != nil && toLower(*job.WorkspaceType) == "remote" {
		isRemote = true
	}
	if !isRemote && len(job.Locations) > 0 && job.Locations[0].City != nil {
		isRemote = caseInsensitiveContains(*job.Locations[0].City, "remote")
	}

	// Job URL
	jobURL := ""
	if job.URLs != nil && job.URLs.Ad != nil && *job.URLs.Ad != "" {
		jobURL = *job.URLs.Ad
	}
	if jobURL == "" && job.Slug != nil && *job.Slug != "" {
		jobURL = fmt.Sprintf("https://jobs.jobylon.com/jobs/%s/", *job.Slug)
	} else if jobURL == "" && job.ID != nil {
		jobURL = fmt.Sprintf("https://jobs.jobylon.com/jobs/%d/", *job.ID)
	}

	// Date posted
	var datePosted *time.Time
	if job.FromDate != nil && *job.FromDate != "" {
		datePosted = util.ParseDatePosted(*job.FromDate)
	}

	// Skills
	skills := make([]string, 0)
	if len(job.Skills) > 0 {
		for _, s := range job.Skills {
			if s.Label != nil && *s.Label != "" {
				skills = append(skills, *s.Label)
			}
		}
	}

	// Company name
	companyName := seed
	if job.Company != nil && job.Company.Name != nil && *job.Company.Name != "" {
		companyName = *job.Company.Name
	}

	return model.JobPost{
		ID:            ats.BuildID("jbl", seed, jobID),
		Title:         title,
		CompanyName:   companyName,
		JobURL:        jobURL,
		Location:      location,
		IsRemote:      isRemote,
		Description:   description,
		DatePosted:    datePosted,
		Site:          string(s.SiteName()),
		Department:    safeString(job.Department),
		JobType:       safeString(job.EmploymentType),
		Skills:        skills,
	}
}

func safeString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
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

func caseInsensitiveContains(s, substr string) bool {
	if len(s) < len(substr) {
		return false
	}
	sLower := toLower(s)
	subLower := toLower(substr)
	for i := 0; i <= len(sLower)-len(subLower); i++ {
		if sLower[i:i+len(subLower)] == subLower {
			return true
		}
	}
	return false
}
