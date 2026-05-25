package jobstreet

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/util"
)

// SEEK v5 API constants.
const (
	apiURL         = "https://www.seek.com.au/api/jobsearch/v5/search"
	defaultSiteKey = "MY-Main"
	maxPages       = 10
	rateLimitDelay = 333 * time.Millisecond // ~3 req/s
	resultsPerPage = 30
)

// salaryRE parses salaryLabel like "RM 5,000 – RM 8,000 per month" or
// "MYR 5,000 - MYR 8,000 per month".
var salaryRE = regexp.MustCompile(`([A-Z]{2,})?\s*\$?([\d,]+)\s*[–-]\s*([A-Z]{2,})?\s*\$?([\d,]+)`)

// intervalRE detects the pay interval from a salaryLabel.
var intervalRE = regexp.MustCompile(`(?i)(per\s+year|per\s+month|per\s+hour|per\s+week|per\s+day|annum|annual|yearly|monthly|hourly)`)

// --- SEEK v5 API response types ---

type seekResponse struct {
	Data       []seekJob `json:"data"`
	TotalCount int       `json:"totalCount"`
}

type seekJob struct {
	ID               string            `json:"id"`
	Title            string            `json:"title"`
	Advertiser       *seekAdvertiser   `json:"advertiser,omitempty"`
	CompanyName      string            `json:"companyName,omitempty"`
	Teaser           string            `json:"teaser,omitempty"`
	ListingDate      string            `json:"listingDate,omitempty"`
	Locations        []seekLocation    `json:"locations,omitempty"`
	SalaryLabel      string            `json:"salaryLabel,omitempty"`
	WorkTypes        []string          `json:"workTypes,omitempty"`
	WorkArrangements *seekWorkArr      `json:"workArrangements,omitempty"`
	Classifications []seekClassGroup  `json:"classifications,omitempty"`
	BulletPoints    []string          `json:"bulletPoints,omitempty"`
	Branding        *seekBranding     `json:"branding,omitempty"`
}

type seekAdvertiser struct {
	Description string `json:"description"`
}

type seekLocation struct {
	Label       string `json:"label"`
	CountryCode string `json:"countryCode,omitempty"`
}

type seekWorkArr struct {
	Data        []any  `json:"data,omitempty"`
	DisplayText string `json:"displayText,omitempty"`
}

type seekClassGroup struct {
	Classification    *seekClass `json:"classification,omitempty"`
	SubClassification *seekClass `json:"subclassification,omitempty"`
}

type seekClass struct {
	Description string `json:"description"`
}

type seekBranding struct {
	SerpLogoURL string `json:"serpLogoUrl,omitempty"`
}

// Scraper scrapes JobStreet via the SEEK v5 REST API.
type Scraper struct {
	client  *http.Client
	apiURL  string
}

// New creates a new JobStreet scraper with the given HTTP client.
func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Retries: 3, Timeout: 25 * time.Second})
	}
	return &Scraper{client: client, apiURL: apiURL}
}

// NewWithURLs creates a scraper with an overridable endpoint (used in tests).
func NewWithURLs(client *http.Client, endpoint string) *Scraper {
	s := New(client)
	if strings.TrimSpace(endpoint) != "" {
		s.apiURL = strings.TrimSpace(endpoint)
	}
	return s
}

// SiteName returns the site identifier.
func (s *Scraper) SiteName() model.Site { return model.SiteJobStreet }

// Scrape fetches job listings from the SEEK v5 search API with pagination.
func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	wanted := input.ResultsWanted
	if wanted <= 0 {
		wanted = 15
	}

	util.Debug("scraper_jobstreet_start", map[string]any{
		"search_term":    input.SearchTerm,
		"location":       input.Location,
		"results_wanted": wanted,
	})

	jobs := make([]model.JobPost, 0, wanted)
	page := 1

	for len(jobs) < wanted && page <= maxPages {
		select {
		case <-ctx.Done():
			util.Debug("scraper_jobstreet_cancelled", map[string]any{"jobs_found": len(jobs)})
			return jobs, ctx.Err()
		default:
		}

		body, err := s.fetchPage(ctx, input.SearchTerm, input.Location, page, wanted)
		if err != nil {
			util.Warn("scraper_jobstreet_fetch_error", map[string]any{"page": page, "err": err.Error()})
			return nil, fmt.Errorf("jobstreet page %d: %w", page, err)
		}

		pageJobs, total, err := parseResponse(body)
		if err != nil {
			util.Warn("scraper_jobstreet_parse_error", map[string]any{"page": page, "err": err.Error()})
			return nil, fmt.Errorf("jobstreet parse page %d: %w", page, err)
		}

		if len(pageJobs) == 0 {
			util.Debug("scraper_jobstreet_no_more_results", map[string]any{"page": page})
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

		// Stop if we've exhausted all results.
		if len(jobs) >= total || len(pageJobs) < resultsPerPage {
			break
		}

		page++
		if err := util.SleepWithContext(ctx, rateLimitDelay); err != nil {
			return jobs, err
		}
	}

	if !util.HasMeaningfulJobs(jobs) {
		return nil, fmt.Errorf("jobstreet no parseable jobs")
	}
	if len(jobs) > wanted {
		jobs = jobs[:wanted]
	}

	util.Debug("scraper_jobstreet_done", map[string]any{"jobs": len(jobs)})
	return jobs, nil
}

// fetchPage makes a GET request to the SEEK v5 API for a given page.
func (s *Scraper) fetchPage(ctx context.Context, searchTerm, location string, page, wantedSize int) ([]byte, error) {
	u, err := url.Parse(s.apiURL)
	if err != nil {
		return nil, fmt.Errorf("jobstreet parse url: %w", err)
	}

	q := u.Query()
	if v := strings.TrimSpace(searchTerm); v != "" {
		q.Set("keywords", v)
	}
	q.Set("pagesize", fmt.Sprintf("%d", wantedSize))
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
	req.Header.Set("Referer", "https://www.jobstreet.com/")
	req.Header.Set("Origin", "https://www.jobstreet.com")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("jobstreet request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("jobstreet status %d", resp.StatusCode)
	}

	body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("jobstreet read: %w", err)
	}
	return body, nil
}

// parseResponse extracts jobs from the SEEK v5 JSON response.
func parseResponse(raw []byte) ([]seekJob, int, error) {
	var resp seekResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, 0, fmt.Errorf("jobstreet decode: %w", err)
	}
	if resp.Data == nil {
		return nil, 0, nil
	}
	return resp.Data, resp.TotalCount, nil
}

// mapJob converts a SEEK v5 API job to a model.JobPost.
func mapJob(j seekJob) *model.JobPost {
	if strings.TrimSpace(j.ID) == "" || strings.TrimSpace(j.Title) == "" {
		return nil
	}

	// Resolve company name: advertiser.description -> companyName.
	companyName := strings.TrimSpace(j.CompanyName)
	if companyName == "" && j.Advertiser != nil {
		companyName = strings.TrimSpace(j.Advertiser.Description)
	}

	// Build job URL.
	jobURL := fmt.Sprintf("https://www.jobstreet.com/job/%s", strings.TrimSpace(j.ID))

	// Parse location.
	var locationCity string
	if len(j.Locations) > 0 {
		locationCity = strings.TrimSpace(j.Locations[0].Label)
	}

	// Parse listing date (RFC3339 format).
	var datePosted *time.Time
	if v := strings.TrimSpace(j.ListingDate); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			datePosted = &t
		}
	}

	// Determine remote status from workArrangements.
	isRemote := false
	if j.WorkArrangements != nil {
		text := strings.ToLower(strings.TrimSpace(j.WorkArrangements.DisplayText))
		isRemote = strings.Contains(text, "remote")
	}

	// Determine job type (workTypes[0]).
	jobType := ""
	if len(j.WorkTypes) > 0 {
		jobType = strings.TrimSpace(j.WorkTypes[0])
	}

	// Parse compensation from salaryLabel.
	comp := parseSalary(j.SalaryLabel)

	// Department from classification > subclassification.
	department := ""
	// Industry from classification > classification.description.
	industry := ""
	if len(j.Classifications) > 0 {
		if j.Classifications[0].SubClassification != nil {
			department = strings.TrimSpace(j.Classifications[0].SubClassification.Description)
		}
		if j.Classifications[0].Classification != nil {
			industry = strings.TrimSpace(j.Classifications[0].Classification.Description)
		}
	}

	// Company logo from branding.
	logoURL := ""
	if j.Branding != nil {
		logoURL = strings.TrimSpace(j.Branding.SerpLogoURL)
	}

	// Description: teaser + bulletPoints.
	description := strings.TrimSpace(j.Teaser)
	if len(j.BulletPoints) > 0 {
		if description != "" {
			description += "\n"
		}
		description += strings.Join(j.BulletPoints, "\n")
	}

	return &model.JobPost{
		ID:             "jobstreet-" + strings.TrimSpace(j.ID),
		Title:          strings.TrimSpace(j.Title),
		CompanyName:    companyName,
		JobURL:         jobURL,
		Location:       model.Location{City: locationCity},
		Description:    description,
		DatePosted:     datePosted,
		IsRemote:       isRemote,
		JobType:        jobType,
		Compensation:   comp,
		Department:     department,
		Industry:       industry,
		CompanyLogoURL: logoURL,
	}
}

// parseSalary extracts compensation data from a SEEK salaryLabel.
func parseSalary(label string) *model.Compensation {
	label = strings.TrimSpace(label)
	if label == "" || len(label) > 200 {
		return nil
	}

	m := salaryRE.FindStringSubmatch(label)
	if len(m) < 5 {
		return nil
	}

	currency := m[1]
	if currency == "" {
		currency = m[3]
	}
	if currency == "" {
		currency = "MYR"
	}

	minRaw := strings.ReplaceAll(m[2], ",", "")
	maxRaw := strings.ReplaceAll(m[4], ",", "")

	var minF, maxF float64
	if _, err := fmt.Sscanf(minRaw, "%f", &minF); err != nil || minF <= 0 {
		return nil
	}
	if _, err := fmt.Sscanf(maxRaw, "%f", &maxF); err != nil || maxF <= 0 {
		return nil
	}

	// Detect interval from label.
	interval := model.IntervalYearly // default
	if im := intervalRE.FindStringSubmatch(label); len(im) > 1 {
		switch strings.ToLower(strings.TrimSpace(im[1])) {
		case "per month", "monthly":
			interval = model.IntervalMonthly
		case "per hour", "hourly":
			interval = model.IntervalHourly
		case "per week", "weekly":
			interval = model.IntervalWeekly
		case "per day", "daily":
			interval = model.IntervalDaily
		}
	}

	return &model.Compensation{
		Interval:  interval,
		MinAmount: &minF,
		MaxAmount: &maxF,
		Currency:  currency,
	}
}
