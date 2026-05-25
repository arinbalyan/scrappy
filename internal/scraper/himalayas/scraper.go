package himalayas

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

const defaultAPI = "https://himalayas.app/jobs/api"

	// Default page size per the TypeScript reference.
	const defaultPageSize = 20
	const maxPages = 10

// --- API response types ---

// himalayasJob maps a single job from the Himalayas API.
type himalayasJob struct {
	GUID                 string   `json:"guid"`
	Title                string   `json:"title"`
	CompanyName          string   `json:"companyName"`
	CompanyLogo          string   `json:"companyLogo"`
	EmploymentType       string   `json:"employmentType"`
	MinSalary            *float64 `json:"minSalary"`
	MaxSalary            *float64 `json:"maxSalary"`
	Seniority            []string `json:"seniority"`
	Currency             string   `json:"currency"`
	LocationRestrictions []string `json:"locationRestrictions"`
	Description          string   `json:"description"`
	PubDate              float64  `json:"pubDate"` // Unix timestamp in seconds
	ApplicationLink      string   `json:"applicationLink"`
}

// apiResponse maps the top-level Himalayas API response envelope.
type apiResponse struct {
	Jobs        []himalayasJob `json:"jobs"`
	TotalCount  int            `json:"totalCount"`
	Offset      int            `json:"offset"`
	Limit       int            `json:"limit"`
}

// Scraper fetches jobs from the Himalayas public API.
type Scraper struct {
	client  *http.Client
	baseURL string
}

// New creates a new Himalayas scraper. If client is nil a default one is used.
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
func (s *Scraper) SiteName() model.Site { return model.SiteHimalayas }

// Scrape fetches jobs from the Himalayas API using limit/offset pagination.
// The API returns an object with a "jobs" array and pagination metadata.
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

		pageSize := defaultPageSize
		if remaining := wanted - len(jobs); remaining < pageSize {
			pageSize = remaining
		}

		// Build request URL with limit and offset (matching TS source).
		u, _ := url.Parse(s.baseURL)
		q := url.Values{}
		q.Set("limit", strconv.Itoa(pageSize))
		q.Set("offset", strconv.Itoa(offset))
		u.RawQuery = q.Encode()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
		if err != nil {
			return nil, fmt.Errorf("himalayas: build request: %w", err)
		}
		req.Header.Set("Accept", "application/json")

		resp, err := s.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("himalayas: request: %w", err)
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			resp.Body.Close()
			return nil, fmt.Errorf("himalayas: status %d", resp.StatusCode)
		}

		body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("himalayas: read: %w", err)
		}

		var parsed apiResponse
		if err := json.Unmarshal(body, &parsed); err != nil {
			return nil, fmt.Errorf("himalayas: decode: %w", err)
		}

		if len(parsed.Jobs) == 0 {
			break
		}

		for _, raw := range parsed.Jobs {
			if len(jobs) >= wanted {
				break
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
		return nil, fmt.Errorf("himalayas: no parseable jobs")
	}

	return jobs, nil
}

// mapJob converts a raw Himalayas API job to a model.JobPost.
func mapJob(raw himalayasJob) model.JobPost {
	job := model.JobPost{
		ID:      "himalayas-" + raw.GUID,
		Title:   strings.TrimSpace(raw.Title),
		Site:    string(model.SiteHimalayas),
		IsRemote: true, // Himalayas is a remote-only board
	}

	// Company
	job.CompanyName = strings.TrimSpace(raw.CompanyName)
	job.CompanyLogoURL = strings.TrimSpace(raw.CompanyLogo)

	// Job URL
	if raw.ApplicationLink != "" {
		job.JobURL = strings.TrimSpace(raw.ApplicationLink)
	} else {
		job.JobURL = "https://himalayas.app/jobs"
	}

	// Description
	job.Description = strings.TrimSpace(raw.Description)

	// Location from locationRestrictions (first entry)
	if len(raw.LocationRestrictions) > 0 {
		if loc := strings.TrimSpace(raw.LocationRestrictions[0]); loc != "" {
			job.Location = model.Location{City: loc}
		}
	}

	// Employment type
	job.JobType = strings.ToLower(strings.TrimSpace(raw.EmploymentType))

	// Compensation from minSalary / maxSalary
	if raw.MinSalary != nil || raw.MaxSalary != nil {
		curr := strings.TrimSpace(raw.Currency)
		if curr == "" {
			curr = "USD"
		}
		job.Compensation = &model.Compensation{
			Interval:  model.IntervalYearly,
			MinAmount: raw.MinSalary,
			MaxAmount: raw.MaxSalary,
			Currency:  curr,
		}
	}

	// Seniority / job level
	if len(raw.Seniority) > 0 {
		job.Seniority = strings.Join(raw.Seniority, ", ")
		job.JobLevel = strings.Join(raw.Seniority, ", ")
	}

	// DatePosted from pubDate (Unix timestamp in seconds)
	if raw.PubDate > 0 {
		secs := int64(raw.PubDate)
		t := time.Unix(secs, 0)
		job.DatePosted = &t
	}

	return job
}
