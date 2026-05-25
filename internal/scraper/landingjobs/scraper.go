package landingjobs

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/util"
)

const defaultAPI = "https://landing.jobs/api/v1/jobs"

// Default page size and max pages from the TypeScript reference.
const defaultPageSize = 50
const maxPages = 5

// --- API response types ---

// landingJob maps a single job from the Landing.jobs API.
type landingJob struct {
	ID           int       `json:"id"`
	Title        string    `json:"title"`
	City         string    `json:"city,omitempty"`
	CountryCode  string    `json:"country_code,omitempty"`
	CountryName  string    `json:"country_name,omitempty"`
	CurrencyCode string    `json:"currency_code,omitempty"`
	SalaryLow    *float64  `json:"salary_low,omitempty"`
	SalaryHigh   *float64  `json:"salary_high,omitempty"`
	Type         string    `json:"type,omitempty"`
	Remote       bool      `json:"remote,omitempty"`
	Description  string    `json:"role_description,omitempty"`
	Tags         []string  `json:"tags,omitempty"`
	PublishedAt  string    `json:"published_at,omitempty"`
}

// Scraper fetches jobs from the Landing.jobs public API.
type Scraper struct {
	client  *http.Client
	baseURL string
}

// New creates a new Landing.jobs scraper. If client is nil a default one is used.
func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Timeout: 20 * time.Second})
	}
	return &Scraper{client: client, baseURL: defaultAPI}
}

// NewWithAPIURL creates a new scraper with a custom endpoint (used in tests).
func NewWithAPIURL(client *http.Client, endpoint string) *Scraper {
	s := New(client)
	if strings.TrimSpace(endpoint) != "" {
		s.baseURL = endpoint
	}
	return s
}

// SiteName returns the site identifier.
func (s *Scraper) SiteName() model.Site { return model.SiteLandingJobs }

// Scrape fetches jobs from Landing.jobs using offset-based pagination.
// The API returns a bare JSON array of jobs.
func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{
		"site":           s.SiteName(),
		"results_wanted": input.ResultsWanted,
		"search_term":    input.SearchTerm,
	})

	wanted := input.ResultsWanted
	if wanted <= 0 {
		wanted = 25
	}

	jobs := make([]model.JobPost, 0, wanted)
	offset := 0
	page := 0

	for len(jobs) < wanted && page < maxPages {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// Always request the default page size from the API.
		pageSize := defaultPageSize
		if remaining := wanted - len(jobs); remaining < pageSize {
			pageSize = remaining
		}

		// Build request URL with offset and limit.
		u, _ := url.Parse(s.baseURL)
		q := url.Values{}
		q.Set("offset", strconv.Itoa(offset))
		q.Set("limit", strconv.Itoa(pageSize))
		u.RawQuery = q.Encode()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
		if err != nil {
			return nil, fmt.Errorf("landingjobs: build request: %w", err)
		}
		req.Header.Set("Accept", "application/json")

		resp, err := s.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("landingjobs: request: %w", err)
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			resp.Body.Close()
			return nil, fmt.Errorf("landingjobs: status %d", resp.StatusCode)
		}

		body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("landingjobs: read: %w", err)
		}

		var rawJobs []landingJob
		if err := json.Unmarshal(body, &rawJobs); err != nil {
			return nil, fmt.Errorf("landingjobs: decode: %w", err)
		}

		if len(rawJobs) == 0 {
			break
		}

		term := strings.ToLower(strings.TrimSpace(input.SearchTerm))

		for _, raw := range rawJobs {
			if len(jobs) >= wanted {
				break
			}

			// Client-side search filter (matching TS reference).
			if term != "" && !matchesSearch(raw, term) {
				continue
			}

			job := mapJob(raw)
			if strings.TrimSpace(job.Title) == "" {
				continue
			}
			jobs = append(jobs, job)
		}

		offset += defaultPageSize
		page++
	}

	util.Debug("scraper_done", map[string]any{
		"site": s.SiteName(),
		"jobs": len(jobs),
	})

	if !util.HasMeaningfulJobs(jobs) {
		return nil, fmt.Errorf("landingjobs: no parseable jobs")
	}

	return jobs, nil
}

// matchesSearch checks whether a job matches the given search term (case-insensitive)
// across title, role_description, and tags.
func matchesSearch(raw landingJob, term string) bool {
	title := strings.ToLower(raw.Title)
	desc := strings.ToLower(raw.Description)
	tags := strings.Join(raw.Tags, " ")
	tags = strings.ToLower(tags)

	return strings.Contains(title, term) ||
		strings.Contains(desc, term) ||
		strings.Contains(tags, term)
}

// mapJob converts a raw Landing.jobs API job to a model.JobPost.
func mapJob(raw landingJob) model.JobPost {
	job := model.JobPost{
		ID:    fmt.Sprintf("landingjobs-%d", raw.ID),
		Title: strings.TrimSpace(raw.Title),
		Site:  string(model.SiteLandingJobs),
	}

	// Job URL
	if raw.ID > 0 {
		job.JobURL = fmt.Sprintf("https://landing.jobs/jobs/%d", raw.ID)
	}

	// Location
	if raw.City != "" || raw.CountryName != "" {
		job.Location = model.Location{
			City:    strings.TrimSpace(raw.City),
			Country: strings.TrimSpace(raw.CountryName),
		}
	}

	// Description
	job.Description = strings.TrimSpace(raw.Description)

	// Remote
	job.IsRemote = raw.Remote

	// Skills from tags
	if len(raw.Tags) > 0 {
		skills := make([]string, 0, len(raw.Tags))
		for _, t := range raw.Tags {
			if s := strings.TrimSpace(t); s != "" {
				skills = append(skills, s)
			}
		}
		job.Skills = skills
	}

	// Compensation from salary_low / salary_high
	if raw.SalaryLow != nil || raw.SalaryHigh != nil {
		curr := strings.TrimSpace(raw.CurrencyCode)
		if curr == "" {
			curr = "EUR"
		}
		job.Compensation = &model.Compensation{
			Interval: model.IntervalYearly,
			MinAmount: raw.SalaryLow,
			MaxAmount: raw.SalaryHigh,
			Currency:  curr,
		}
	}

	// DatePosted from published_at (ISO 8601)
	if raw.PublishedAt != "" {
		if t, err := time.Parse(time.RFC3339, raw.PublishedAt); err == nil {
			job.DatePosted = &t
		} else if t, err := time.Parse("2006-01-02T15:04:05", raw.PublishedAt); err == nil {
			job.DatePosted = &t
		}
	}

	return job
}
