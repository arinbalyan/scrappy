package ismartrecruit

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
	defaultAPIURL = "https://app.ismartrecruit.com/WebsiteJSONAPI"
)

// Job represents a single job from the iSmartRecruit API.
type Job struct {
	JobID       string  `json:"jobId"`
	JobTitle    string  `json:"jobTitle"`
	City        *string `json:"city"`
	Country     *string `json:"country"`
	JobCategory *string `json:"jobCategory"`
	DatePosted  *string `json:"datePosted"`
	Description *string `json:"description"`
	ApplyURL    *string `json:"applyUrl"`
	CompanyName *string `json:"companyName"`
}

// Scraper for iSmartRecruit.
type Scraper struct {
	Client *http.Client
	apiURL string
}

// New creates a new iSmartRecruit scraper.
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
func (s *Scraper) SiteName() model.Site { return model.SiteISmartRecruit }

// Scrape fetches jobs from iSmartRecruit.
func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{"site": s.SiteName(), "results_wanted": input.ResultsWanted, "search_term": input.SearchTerm})

	seeds, src := ats.ResolveSeeds(input.SearchTerm, "SCRAPPY_ISMARTRECRUIT_SEEDS")
	if len(seeds) == 0 {
		return nil, fmt.Errorf("ismartrecruit no seeds: set SCRAPPY_ISMARTRECRUIT_SEEDS or pass a company name in --search")
	}
	util.Debug("ismartrecruit_seeds", map[string]any{"seeds": seeds, "src": src})

	var out []model.JobPost
	seen := map[string]struct{}{}

	for _, seed := range seeds {
		if input.ResultsWanted > 0 && len(out) >= input.ResultsWanted {
			break
		}
		jobs, err := s.fetchJobs(ctx, input, seed)
		if err != nil {
			util.Warn("ismartrecruit_seed_fail", map[string]any{"seed": seed, "err": err.Error()})
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
		return nil, fmt.Errorf("ismartrecruit no parseable jobs")
	}
	return out, nil
}

func (s *Scraper) fetchJobs(ctx context.Context, input model.ScraperInput, seed string) ([]model.JobPost, error) {
	resultsWanted := input.ResultsWanted
	if resultsWanted <= 0 {
		resultsWanted = 100
	}

	url := fmt.Sprintf("%s?apiKey=%s&jobTitle=%s&city=%s&start=0&numOfRecords=%d",
		s.apiURL, seed, input.SearchTerm, input.Location, resultsWanted)

	var jobs []Job
	if err := ats.FetchJSON(ctx, s.Client, url, &jobs); err != nil {
		return nil, fmt.Errorf("ismartrecruit fetch: %w", err)
	}

	out := make([]model.JobPost, 0, len(jobs))
	searchTerm := input.SearchTerm

	for _, job := range jobs {
		if job.JobTitle == "" {
			continue
		}

		// Filter by searchTerm (case-insensitive on title and description)
		if searchTerm != "" {
			title := toLower(job.JobTitle)
			desc := ""
			if job.Description != nil {
				desc = toLower(*job.Description)
			}
			term := toLower(searchTerm)
			if !contains(title, term) && !contains(desc, term) {
				continue
			}
		}

		jp := s.mapJob(job, seed)
		out = append(out, jp)
	}
	return out, nil
}

func (s *Scraper) mapJob(job Job, seed string) model.JobPost {
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
	if job.Country != nil {
		location.Country = *job.Country
	}

	// Remote detection
	isRemote := false
	if job.City != nil {
		isRemote = caseInsensitiveContains(*job.City, "remote")
	}

	// Job URL
	jobURL := ""
	if job.ApplyURL != nil && *job.ApplyURL != "" {
		jobURL = *job.ApplyURL
	}
	if jobURL == "" {
		jobURL = fmt.Sprintf("https://app.ismartrecruit.com/jobDescription/%s", job.JobID)
	}

	// Date posted
	var datePosted *time.Time
	if job.DatePosted != nil && *job.DatePosted != "" {
		datePosted = util.ParseDatePosted(*job.DatePosted)
	}

	// Department
	department := ""
	if job.JobCategory != nil {
		department = *job.JobCategory
	}

	// Company name
	companyName := seed
	if job.CompanyName != nil && *job.CompanyName != "" {
		companyName = *job.CompanyName
	}

	return model.JobPost{
		ID:          ats.BuildID("isr", seed, job.JobID),
		Title:       job.JobTitle,
		CompanyName: companyName,
		JobURL:      jobURL,
		Location:    location,
		IsRemote:    isRemote,
		Description: description,
		DatePosted:  datePosted,
		Site:        string(s.SiteName()),
		Department:  department,
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && len(substr) > 0 && kontext(s, substr)
}

func kontext(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
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
	return contains(sLower, subLower)
}
