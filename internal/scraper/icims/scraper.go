package icims

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
	pageSize = 20
)

// JobListItem represents a single job from the iCIMS gateway JSON endpoint.
type JobListItem struct {
	ID         *string `json:"id"`
	Title      *string `json:"title"`
	URL        *string `json:"url"`
	Location   *string `json:"location"`
	DatePosted *string `json:"datePosted"`
	Category   *string `json:"category"`
}

// GatewayResponse wraps the iCIMS gateway JSON response.
type GatewayResponse struct {
	Jobs        []JobListItem `json:"jobs"`
	TotalCount  *int          `json:"totalCount"`
}

// Scraper for iCIMS.
type Scraper struct {
	Client *http.Client
	apiURL string // base URL template, %s is the company slug
}

// New creates a new iCIMS scraper.
func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Retries: 2, Timeout: 20 * time.Second})
	}
	return &Scraper{Client: client, apiURL: "https://%s.icims.com/jobs/search"}
}

// NewWithAPIURL creates a scraper with a custom API URL template.
// The %s placeholder will be replaced with the company slug.
func NewWithAPIURL(client *http.Client, apiURL string) *Scraper {
	s := New(client)
	if apiURL != "" {
		s.apiURL = apiURL
	}
	return s
}

// SiteName returns the site identifier.
func (s *Scraper) SiteName() model.Site { return model.SiteICIMS }

// Scrape fetches jobs from iCIMS.
func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{"site": s.SiteName(), "results_wanted": input.ResultsWanted, "search_term": input.SearchTerm})

	seeds, src := ats.ResolveSeeds(input.SearchTerm, "SCRAPPY_ICIMS_SEEDS")
	if len(seeds) == 0 {
		return nil, fmt.Errorf("icims no seeds: set SCRAPPY_ICIMS_SEEDS or pass a company name in --search")
	}
	util.Debug("icims_seeds", map[string]any{"seeds": seeds, "src": src})

	wanted := input.ResultsWanted
	if wanted <= 0 {
		wanted = 100
	}

	seen := map[string]struct{}{}
	var mu sync.Mutex

	fetchFn := func(ctx context.Context, slug string) ([]model.JobPost, error) {
		jobs, err := s.fetchGateway(ctx, input, slug)
		if err != nil {
			util.Warn("icims_seed_fail", map[string]any{"seed": slug, "err": err.Error()})
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
		return nil, fmt.Errorf("icims no parseable jobs")
	}
	return results, nil
}

func (s *Scraper) buildGatewayURL(company string, offset int) string {
	// If apiURL already contains the company slug (contains %s), expand it.
	// Otherwise use as-is (for test servers).
	base := s.apiURL
	if baseContainsSlug(base) {
		base = fmt.Sprintf(base, company)
	}
	return fmt.Sprintf("%s?pr=%d&schemaId=&o=%d&mode=job&iis=Internet", base, offset, offset)
}

func baseContainsSlug(u string) bool {
	for i := 0; i < len(u)-1; i++ {
		if u[i] == '%' && u[i+1] == 's' {
			return true
		}
	}
	return false
}

func (s *Scraper) fetchGateway(ctx context.Context, input model.ScraperInput, company string) ([]model.JobPost, error) {
	resultsWanted := input.ResultsWanted
	if resultsWanted <= 0 {
		resultsWanted = 100
	}

	var out []model.JobPost
	offset := 0

	for len(out) < resultsWanted {
		url := s.buildGatewayURL(company, offset)
		util.Debug("icims_fetch_gateway", map[string]any{"company": company, "offset": offset, "url": url})

		var resp GatewayResponse
		if err := ats.FetchJSON(ctx, s.Client, url, &resp); err != nil {
			return out, fmt.Errorf("icims gateway fetch: %w", err)
		}

		if len(resp.Jobs) == 0 {
			break
		}

		for _, j := range resp.Jobs {
			if len(out) >= resultsWanted {
				break
			}
			// Skip empty title jobs
			if j.Title == nil || *j.Title == "" {
				continue
			}
			jp := s.mapJob(j, company)
			out = append(out, jp)
		}

		if len(resp.Jobs) < pageSize {
			break
		}
		offset += pageSize
	}

	return out, nil
}

func (s *Scraper) mapJob(job JobListItem, company string) model.JobPost {
	title := ""
	if job.Title != nil {
		title = *job.Title
	}

	// Job ID
	jobID := ""
	if job.ID != nil {
		jobID = *job.ID
	}

	// Job URL
	jobURL := ""
	if job.URL != nil {
		jobURL = *job.URL
		if len(jobURL) > 0 && jobURL[0] == '/' {
			jobURL = fmt.Sprintf("https://%s.icims.com%s", company, jobURL)
		}
	}
	if jobURL == "" && jobID != "" {
		jobURL = fmt.Sprintf("https://%s.icims.com/jobs/%s/job", company, jobID)
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

	// Date posted
	var datePosted *time.Time
	if job.DatePosted != nil && *job.DatePosted != "" {
		datePosted = util.ParseDatePosted(*job.DatePosted)
	}

	// Department
	department := ""
	if job.Category != nil {
		department = *job.Category
	}

	return model.JobPost{
		ID:         ats.BuildID("icims", company, jobID),
		Title:      title,
		CompanyName: company,
		JobURL:     jobURL,
		Location:   location,
		IsRemote:   isRemote,
		DatePosted: datePosted,
		Site:       string(s.SiteName()),
		Department: department,
	}
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
