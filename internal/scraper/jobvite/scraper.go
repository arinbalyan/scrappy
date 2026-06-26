package jobvite

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/scraper/ats"
	"github.com/arinbalyan/scrappy/internal/util"
)

const (
	defaultAPIURL = "https://jobs.jobvite.com/api/v2/job-feed"
)

// JobviteResponse wraps the API response.
type JobviteResponse struct {
	Requisitions []Job `json:"requisitions"`
	Total        *int  `json:"total"`
}

// Job represents a single job from Jobvite.
type Job struct {
	EID             *string `json:"eId"`
	Title           *string `json:"title"`
	Department      *string `json:"department"`
	Category        *string `json:"category"`
	Location        *string `json:"location"`
	City            *string `json:"city"`
	State           *string `json:"state"`
	Country         *string `json:"country"`
	Type            *string `json:"type"`
	Date            *string `json:"date"`
	Description     *string `json:"description"`
	BriefDescription *string `json:"briefDescription"`
	ApplyURL        *string `json:"applyUrl"`
	DetailURL       *string `json:"detailUrl"`
	RequisitionID   *string `json:"requisitionId"`
}

// Scraper for Jobvite.
type Scraper struct {
	Client *http.Client
	apiURL string
}

// New creates a new Jobvite scraper.
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
func (s *Scraper) SiteName() model.Site { return model.SiteJobvite }

// Scrape fetches jobs from Jobvite.
func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{"site": s.SiteName(), "results_wanted": input.ResultsWanted, "search_term": input.SearchTerm})

	seeds, src := ats.ResolveSeeds(input.SearchTerm, "SCRAPPY_JOBVITE_SEEDS")
	if len(seeds) == 0 {
		return nil, fmt.Errorf("jobvite no seeds: set SCRAPPY_JOBVITE_SEEDS or pass a company name in --search")
	}
	util.Debug("jobvite_seeds", map[string]any{"seeds": seeds, "src": src})

	wanted := input.ResultsWanted
	if wanted <= 0 {
		wanted = 100
	}

	seen := map[string]struct{}{}
	var mu sync.Mutex

	fetchFn := func(ctx context.Context, slug string) ([]model.JobPost, error) {
		jobs, err := s.fetchJobs(ctx, input, slug)
		if err != nil {
			util.Warn("jobvite_seed_fail", map[string]any{"seed": slug, "err": err.Error()})
			return nil, err
		}
		var result []model.JobPost
		for _, jp := range jobs {
			mu.Lock()
			if _, ok := seen[jp.ID]; ok {
				mu.Unlock()
				continue
			}
			seen[jp.ID] = struct{}{}
			mu.Unlock()
			result = append(result, jp)
		}
		return result, nil
	}

	results := ats.ProcessSeeds(ctx, seeds, 3, wanted, fetchFn)
	if !util.HasMeaningfulJobs(results) {
		return nil, fmt.Errorf("jobvite no parseable jobs")
	}
	return results, nil
}

func (s *Scraper) fetchJobs(ctx context.Context, input model.ScraperInput, seed string) ([]model.JobPost, error) {
	url := fmt.Sprintf("%s/%s", s.apiURL, seed)

	var resp JobviteResponse
	if err := ats.FetchJSON(ctx, s.Client, url, &resp); err != nil {
		return nil, fmt.Errorf("jobvite fetch: %w", err)
	}

	out := make([]model.JobPost, 0, len(resp.Requisitions))
	resultsWanted := input.ResultsWanted
	if resultsWanted <= 0 {
		resultsWanted = 100
	}

	for _, job := range resp.Requisitions {
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

	// Job ID
	jobID := ""
	if job.EID != nil {
		jobID = *job.EID
	} else if job.RequisitionID != nil {
		jobID = *job.RequisitionID
	}

	// Description
	description := ""
	rawDesc := ""
	if job.Description != nil && *job.Description != "" {
		rawDesc = *job.Description
	} else if job.BriefDescription != nil && *job.BriefDescription != "" {
		rawDesc = *job.BriefDescription
	}
	if rawDesc != "" {
		description = util.StripHTML(rawDesc)
	}

	// Location
	location := model.Location{}
	if job.City != nil {
		location.City = *job.City
	}
	if job.State != nil {
		location.State = *job.State
	}
	if job.Country != nil {
		location.Country = *job.Country
	}

	// Remote detection
	isRemote := false
	if job.Location != nil {
		isRemote = caseInsensitiveContains(*job.Location, "remote")
	}

	// Job URL
	jobURL := ""
	if job.DetailURL != nil && *job.DetailURL != "" {
		jobURL = *job.DetailURL
	} else if job.ApplyURL != nil && *job.ApplyURL != "" {
		jobURL = *job.ApplyURL
	}
	if jobURL == "" {
		jobURL = fmt.Sprintf("https://jobs.jobvite.com/%s/job/%s", seed, jobID)
	}

	// Date posted
	var datePosted *time.Time
	if job.Date != nil && *job.Date != "" {
		datePosted = util.ParseDatePosted(*job.Date)
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
	if job.Type != nil {
		employmentType = *job.Type
	}

	return model.JobPost{
		ID:          ats.BuildID("jv", seed, jobID),
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
