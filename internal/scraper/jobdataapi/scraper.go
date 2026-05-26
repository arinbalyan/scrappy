package jobdataapi

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

const apiURL = "https://jobdataapi.com/api/jobs/"

// Scraper fetches jobs from the JobDataAPI REST API.
type Scraper struct {
	client *http.Client
	apiURL string
	apiKey string
}

// New creates a new JobDataAPI scraper. If client is nil a default one is used.
func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Timeout: 20 * time.Second})
	}
	return &Scraper{client: client, apiURL: apiURL}
}

// NewWithAPIURL creates a new scraper with a custom endpoint (used in tests).
func NewWithAPIURL(client *http.Client, endpoint string) *Scraper {
	s := New(client)
	if strings.TrimSpace(endpoint) != "" {
		s.apiURL = endpoint
	}
	return s
}

// SiteName returns the site identifier.
func (s *Scraper) SiteName() model.Site { return model.SiteJobDataAPI }

// Scrape fetches jobs from the JobDataAPI. Supports pagination via page/page_size params.
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
	pageSize := wanted
	if pageSize > 25 {
		pageSize = 25
	}

	var allJobs []model.JobPost
	seenIDs := make(map[string]bool)
	page := 1

	for page <= 5 {
		select {
		case <-ctx.Done():
			return allJobs, ctx.Err()
		default:
		}

		if len(allJobs) >= wanted {
			break
		}

		u, err := url.Parse(s.apiURL)
		if err != nil {
			return nil, fmt.Errorf("jobdataapi: parse url: %w", err)
		}
		q := u.Query()
		q.Set("page", strconv.Itoa(page))
		q.Set("page_size", strconv.Itoa(pageSize))
		if input.SearchTerm != "" {
			q.Set("title", input.SearchTerm)
		}
		if input.Location != "" {
			q.Set("location_search", input.Location)
		}
		u.RawQuery = q.Encode()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
		if err != nil {
			return nil, fmt.Errorf("jobdataapi: build request: %w", err)
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "Mozilla/5.0")

		resp, err := s.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("jobdataapi: request: %w", err)
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			resp.Body.Close()
			return nil, fmt.Errorf("jobdataapi: status %d", resp.StatusCode)
		}

		body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("jobdataapi: read: %w", err)
		}

		var apiResp apiResponse
		if err := json.Unmarshal(body, &apiResp); err != nil {
			return nil, fmt.Errorf("jobdataapi: decode: %w", err)
		}

		if len(apiResp.Results) == 0 {
			break
		}

		remaining := wanted - len(allJobs)
		for _, raw := range apiResp.Results {
			if remaining <= 0 {
				break
			}

			jobID := fmt.Sprintf("jobdataapi-%d", raw.ID)
			if seenIDs[jobID] {
				continue
			}
			seenIDs[jobID] = true

			job := mapJob(raw)
			if job != nil {
				allJobs = append(allJobs, *job)
				remaining--
			}
		}

		if apiResp.Next == "" {
			break
		}
		page++
	}

	util.Debug("scraper_done", map[string]any{
		"site": s.SiteName(),
		"jobs": len(allJobs),
	})

	if !util.HasMeaningfulJobs(allJobs) {
		return nil, fmt.Errorf("jobdataapi: no parseable jobs")
	}
	return allJobs, nil
}

// apiResponse maps the JobDataAPI paginated response.
type apiResponse struct {
	Count    int        `json:"count"`
	Next     string     `json:"next"`
	Previous string     `json:"previous"`
	Results  []jobItem  `json:"results"`
}

// jobItem maps a single JobDataAPI job.
type jobItem struct {
	ID              int        `json:"id"`
	Title           string     `json:"title"`
	Slug            string     `json:"slug"`
	Company         *company   `json:"company,omitempty"`
	HasRemote       bool       `json:"has_remote"`
	Location        *loc       `json:"location,omitempty"`
	Description     string     `json:"description,omitempty"`
	ApplicationURL  string     `json:"application_url,omitempty"`
	JobType         string     `json:"job_type,omitempty"`
	SalaryMin       *float64   `json:"salary_min,omitempty"`
	SalaryMax       *float64   `json:"salary_max,omitempty"`
	SalaryCurrency  string     `json:"salary_currency,omitempty"`
	DatePosted      string     `json:"date_posted,omitempty"`
	Tags            []string   `json:"tags,omitempty"`
}

type company struct {
	Name string `json:"name"`
	Logo string `json:"logo,omitempty"`
}

type loc struct {
	City    string `json:"city,omitempty"`
	Country string `json:"country,omitempty"`
}

// mapJob converts a raw JobDataAPI job item to a JobPost.
func mapJob(raw jobItem) *model.JobPost {
	title := strings.TrimSpace(raw.Title)
	if title == "" {
		return nil
	}

	// Build job URL
	jobURL := strings.TrimSpace(raw.ApplicationURL)
	if jobURL == "" && raw.Slug != "" {
		jobURL = fmt.Sprintf("https://jobdataapi.com/jobs/%s/", raw.Slug)
	}

	// Build location
	loc := model.Location{}
	if raw.Location != nil {
		loc.City = strings.TrimSpace(raw.Location.City)
		loc.Country = strings.TrimSpace(raw.Location.Country)
	}

	// Build compensation
	var compensation *model.Compensation
	hasMin := raw.SalaryMin != nil && *raw.SalaryMin != 0
	hasMax := raw.SalaryMax != nil && *raw.SalaryMax != 0
	if hasMin || hasMax {
		currency := strings.TrimSpace(raw.SalaryCurrency)
		if currency == "" {
			currency = "USD"
		}
		compensation = &model.Compensation{
			Interval: model.IntervalYearly,
			Currency: currency,
		}
		if hasMin {
			compensation.MinAmount = raw.SalaryMin
		}
		if hasMax {
			compensation.MaxAmount = raw.SalaryMax
		}
	}

	// Parse date
	var datePosted *time.Time
	if raw.DatePosted != "" {
		datePosted = util.ParseDatePosted(raw.DatePosted)
	}

	// Company name
	companyName := ""
	if raw.Company != nil {
		companyName = strings.TrimSpace(raw.Company.Name)
	}

	// Skills from tags
	skills := make([]string, 0)
	if len(raw.Tags) > 0 {
		for _, t := range raw.Tags {
			if strings.TrimSpace(t) != "" {
				skills = append(skills, strings.TrimSpace(t))
			}
		}
	}

	return &model.JobPost{
		ID:           fmt.Sprintf("jobdataapi-%d", raw.ID),
		Title:        title,
		CompanyName:  companyName,
		JobURL:       jobURL,
		Description:  strings.TrimSpace(raw.Description),
		Location:     loc,
		IsRemote:     raw.HasRemote,
		Compensation: compensation,
		DatePosted:   datePosted,
		Skills:       skills,
		Site:         string(model.SiteJobDataAPI),
		ApplyMethod:  "external_url",
	}
}
