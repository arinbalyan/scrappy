package jobsdb

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/util"
)

const (
	apiURL          = "https://www.jobsdb.com/api/chalice-search/v4/search"
	defaultSiteKey  = "SG-Main"
	maxPages        = 10
	rateLimitDelay  = 333 * time.Millisecond // ~3 req/s
	resultsPerPage  = 30
	searchDateFmt   = "2006-01-02T15:04:05"
)

// jobsdbResponse is the top-level API response wrapper.
type jobsdbResponse struct {
	Data []jobsdbJob `json:"data"`
}

// jobsdbJob is a single job posting from the JobsDB Chalice API.
type jobsdbJob struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	CompanyName string `json:"companyName"`
	Location    string `json:"location"`
	Salary      string `json:"salary"`
	JobType     string `json:"jobType"`
	ListingDate string `json:"listingDate"`
	Teaser      string `json:"teaser"`
	JobURL      string `json:"jobUrl"`
	Description string `json:"description"`
	WorkType    string `json:"workType"`
	IsRemote    bool   `json:"isRemote"`
}

// Scraper scrapes JobsDB (jobsdb.com) via their Chalice search API.
type Scraper struct {
	client  *http.Client
	apiURL  string
}

// New creates a new JobsDB scraper with the given HTTP client.
func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Retries: 3, Timeout: 25 * time.Second})
	}
	return &Scraper{client: client, apiURL: apiURL}
}

// NewWithURLs creates a scraper with an overridable API endpoint (used in tests).
func NewWithURLs(client *http.Client, endpoint string) *Scraper {
	s := New(client)
	if strings.TrimSpace(endpoint) != "" {
		s.apiURL = strings.TrimSpace(endpoint)
	}
	return s
}

// SiteName returns the site identifier.
func (s *Scraper) SiteName() model.Site { return model.SiteJobsDB }

// Scrape fetches job listings from the JobsDB Chalice search API with pagination.
func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	wanted := input.ResultsWanted
	if wanted <= 0 {
		wanted = 15
	}

	util.Debug("scraper_jobsdb_start", map[string]any{
		"search_term":    input.SearchTerm,
		"location":       input.Location,
		"results_wanted": wanted,
	})

	jobs := make([]model.JobPost, 0, wanted)
	page := 1

	for len(jobs) < wanted && page <= maxPages {
		select {
		case <-ctx.Done():
			util.Debug("scraper_jobsdb_cancelled", map[string]any{"jobs_found": len(jobs)})
			return jobs, ctx.Err()
		default:
		}

		body, err := s.fetchPage(ctx, input.SearchTerm, input.Location, page)
		if err != nil {
			util.Warn("scraper_jobsdb_fetch_error", map[string]any{"page": page, "err": err.Error()})
			return nil, fmt.Errorf("jobsdb page %d: %w", page, err)
		}

		pageJobs, err := parseResponse(body)
		if err != nil {
			util.Warn("scraper_jobsdb_parse_error", map[string]any{"page": page, "err": err.Error()})
			return nil, fmt.Errorf("jobsdb parse page %d: %w", page, err)
		}

		if len(pageJobs) == 0 {
			util.Debug("scraper_jobsdb_no_more_results", map[string]any{"page": page})
			break
		}

		for _, j := range pageJobs {
			if len(jobs) >= wanted {
				break
			}
			job := mapJob(j)
			if job != nil {
				jobs = append(jobs, *job)
			}
		}

		page++
		// Rate limit: ~3 requests per second
		if err := util.SleepWithContext(ctx, rateLimitDelay); err != nil {
			return jobs, err
		}
	}

	if !util.HasMeaningfulJobs(jobs) {
		return nil, fmt.Errorf("jobsdb no parseable jobs")
	}
	if len(jobs) > wanted {
		jobs = jobs[:wanted]
	}

	util.Debug("scraper_jobsdb_done", map[string]any{"jobs": len(jobs)})
	return jobs, nil
}

// fetchPage makes a GET request to the JobsDB Chalice API for a given page.
func (s *Scraper) fetchPage(ctx context.Context, searchTerm, location string, page int) ([]byte, error) {
	u, err := url.Parse(s.apiURL)
	if err != nil {
		return nil, fmt.Errorf("jobsdb parse url: %w", err)
	}

	q := u.Query()
	if v := strings.TrimSpace(searchTerm); v != "" {
		q.Set("keywords", v)
	}
	q.Set("pageSize", fmt.Sprintf("%d", resultsPerPage))
	q.Set("page", fmt.Sprintf("%d", page))
	q.Set("siteKey", defaultSiteKey)
	if v := strings.TrimSpace(location); v != "" {
		q.Set("where", v)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/129.0.0.0 Safari/537.36")
	req.Header.Set("Referer", "https://www.jobsdb.com/")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("jobsdb request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("jobsdb status %d", resp.StatusCode)
	}

	body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("jobsdb read: %w", err)
	}
	return body, nil
}

// parseResponse extracts jobs from the Chalice API JSON response.
func parseResponse(raw []byte) ([]jobsdbJob, error) {
	var resp jobsdbResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("jobsdb decode: %w", err)
	}
	if resp.Data == nil {
		return nil, nil
	}
	return resp.Data, nil
}

// mapJob converts a raw JobsDB job to a model.JobPost.
func mapJob(j jobsdbJob) *model.JobPost {
	if strings.TrimSpace(j.ID) == "" || strings.TrimSpace(j.Title) == "" {
		return nil
	}

	// Build the job URL.
	var jobURL string
	if v := strings.TrimSpace(j.JobURL); v != "" {
		if strings.HasPrefix(v, "http") {
			jobURL = v
		} else {
			jobURL = "https://www.jobsdb.com" + v
		}
	} else {
		jobURL = fmt.Sprintf("https://www.jobsdb.com/job/%s", j.ID)
	}

	// Parse the listing date.
	var datePosted *time.Time
	if v := strings.TrimSpace(j.ListingDate); v != "" {
		// Try standard ISO format.
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			datePosted = &t
		} else if t, err := time.Parse(searchDateFmt, v); err == nil {
			datePosted = &t
		}
	}

	// Determine remote status.
	isRemote := j.IsRemote
	if !isRemote {
		wt := strings.ToLower(strings.TrimSpace(j.WorkType))
		isRemote = strings.Contains(wt, "remote")
	}

	// Build description from available content.
	description := strings.TrimSpace(j.Description)
	if description == "" {
		description = strings.TrimSpace(j.Teaser)
	}

	// Standardise site-specific fields.
	jobType := strings.TrimSpace(j.JobType)

	return &model.JobPost{
		ID:          "jobsdb-" + j.ID,
		Title:       strings.TrimSpace(j.Title),
		CompanyName: strings.TrimSpace(j.CompanyName),
		JobURL:      jobURL,
		Location:    model.Location{City: strings.TrimSpace(j.Location)},
		Description: description,
		DatePosted:  datePosted,
		IsRemote:    isRemote,
		JobType:     jobType,
	}
}
