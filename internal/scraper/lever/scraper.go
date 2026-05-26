package lever

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
	defaultAPIURL = "https://api.lever.co/v0/postings"
)

// Categories holds job category info from Lever.
type Categories struct {
	Location    *string  `json:"location"`
	Team        *string  `json:"team"`
	Commitment  *string  `json:"commitment"`
	AllLocations []string `json:"allLocations"`
}

// ListItem represents a formatted content block.
type ListItem struct {
	Text    string `json:"text"`
	Content string `json:"content"`
}

// Job represents a single job from Lever.
type Job struct {
	ID                  *string     `json:"id"`
	Text                *string     `json:"text"`
	DescriptionPlain    *string     `json:"descriptionPlain"`
	Description         *string     `json:"description"`
	DescriptionBody     *string     `json:"descriptionBody"`
	DescriptionBodyPlain *string    `json:"descriptionBodyPlain"`
	Additional          *string     `json:"additional"`
	AdditionalPlain     *string     `json:"additionalPlain"`
	Categories          *Categories `json:"categories"`
	CreatedAt           *int64      `json:"createdAt"`
	WorkplaceType       *string     `json:"workplaceType"`
	HostedURL           *string     `json:"hostedUrl"`
	ApplyURL            *string     `json:"applyUrl"`
	Lists               []ListItem  `json:"lists"`
}

// Scraper for Lever.
type Scraper struct {
	Client *http.Client
	apiURL string
}

// New creates a new Lever scraper.
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
func (s *Scraper) SiteName() model.Site { return model.SiteLever }

// Scrape fetches jobs from Lever.
func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{"site": s.SiteName(), "results_wanted": input.ResultsWanted, "search_term": input.SearchTerm})

	seeds, src := ats.ResolveSeeds(input.SearchTerm, "SCRAPPY_LEVER_SEEDS")
	if len(seeds) == 0 {
		return nil, fmt.Errorf("lever no seeds: set SCRAPPY_LEVER_SEEDS or pass a company name in --search")
	}
	util.Debug("lever_seeds", map[string]any{"seeds": seeds, "src": src})

	var out []model.JobPost
	seen := map[string]struct{}{}

	for _, seed := range seeds {
		if input.ResultsWanted > 0 && len(out) >= input.ResultsWanted {
			break
		}
		jobs, err := s.fetchJobs(ctx, input, seed)
		if err != nil {
			util.Warn("lever_seed_fail", map[string]any{"seed": seed, "err": err.Error()})
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
		return nil, fmt.Errorf("lever no parseable jobs")
	}
	return out, nil
}

func (s *Scraper) fetchJobs(ctx context.Context, input model.ScraperInput, seed string) ([]model.JobPost, error) {
	url := fmt.Sprintf("%s/%s?mode=json", s.apiURL, seed)

	var jobs []Job
	if err := ats.FetchJSON(ctx, s.Client, url, &jobs); err != nil {
		return nil, fmt.Errorf("lever fetch: %w", err)
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
		if job.Text == nil || *job.Text == "" {
			continue
		}
		jp := s.mapJob(job, seed)
		out = append(out, jp)
	}
	return out, nil
}

func (s *Scraper) mapJob(job Job, seed string) model.JobPost {
	title := ""
	if job.Text != nil {
		title = *job.Text
	}

	jobID := ""
	if job.ID != nil {
		jobID = *job.ID
	}

	// Build description from available fields
	description := ""
	if job.DescriptionPlain != nil && *job.DescriptionPlain != "" {
		description = *job.DescriptionPlain
	} else if job.DescriptionBodyPlain != nil && *job.DescriptionBodyPlain != "" {
		description = *job.DescriptionBodyPlain
	} else if job.Description != nil && *job.Description != "" {
		description = util.StripHTML(*job.Description)
	} else if job.DescriptionBody != nil && *job.DescriptionBody != "" {
		description = util.StripHTML(*job.DescriptionBody)
	}

	// Append content blocks from lists
	if len(job.Lists) > 0 {
		for _, l := range job.Lists {
			if l.Text != "" || l.Content != "" {
				content := util.StripHTML(l.Content)
				if l.Text != "" {
					description += "\n\n" + l.Text + "\n" + content
				} else {
					description += "\n\n" + content
				}
			}
		}
	}

	// Append additional info
	if job.AdditionalPlain != nil && *job.AdditionalPlain != "" && description != "" {
		description += "\n\n" + *job.AdditionalPlain
	} else if job.AdditionalPlain != nil && *job.AdditionalPlain != "" {
		description = *job.AdditionalPlain
	}

	// Location
	location := model.Location{}
	locationStr := ""
	isRemote := false

	if job.Categories != nil {
		if job.Categories.Location != nil {
			locationStr = *job.Categories.Location
			location.City = locationStr
			if stringsContains(stringsToLower(locationStr), "remote") {
				isRemote = true
			}
		}
	}

	// Workplace type
	if !isRemote && job.WorkplaceType != nil {
		isRemote = stringsContains(stringsToLower(*job.WorkplaceType), "remote")
	}

	// Team / department
	department := ""
	employmentType := ""
	if job.Categories != nil {
		if job.Categories.Team != nil {
			department = *job.Categories.Team
		}
		if job.Categories.Commitment != nil {
			employmentType = *job.Categories.Commitment
		}
	}

	// Job URL
	jobURL := ""
	if job.HostedURL != nil && *job.HostedURL != "" {
		jobURL = *job.HostedURL
	}
	if jobURL == "" {
		jobURL = fmt.Sprintf("https://jobs.lever.co/%s/%s", seed, jobID)
	}

	// Apply URL
	_ = job.ApplyURL

	// Date posted
	var datePosted *time.Time
	if job.CreatedAt != nil && *job.CreatedAt > 0 {
		t := time.UnixMilli(*job.CreatedAt)
		datePosted = &t
	}

	return model.JobPost{
		ID:          ats.BuildID("lever", seed, jobID),
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
		ApplyMethod: "external_url",
	}
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
