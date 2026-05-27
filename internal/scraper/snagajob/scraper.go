package snagajob

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
	searchURL      = "https://www.snagajob.com/api/search"
	pageSize       = 20
	rateLimitDelayMin = 200 * time.Millisecond
	rateLimitDelayMax = 500 * time.Millisecond // 200-500ms jitter
	maxRetries     = 3
)

// Scraper scrapes Snagajob.com using their public search JSON API.
type Scraper struct {
	client    *http.Client
	searchURL string
}

// New creates a new Snagajob scraper with the given HTTP client.
func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Retries: 3, Timeout: 25 * time.Second})
	}
	return &Scraper{client: client, searchURL: searchURL}
}

// NewWithURLs creates a scraper with an overridable endpoint (used in tests).
func NewWithURLs(client *http.Client, endpoint string) *Scraper {
	s := New(client)
	if strings.TrimSpace(endpoint) != "" {
		s.searchURL = strings.TrimSpace(endpoint)
	}
	return s
}

// SiteName returns the site identifier.
func (s *Scraper) SiteName() model.Site { return model.SiteSnagajob }

// Scrape fetches job listings from Snagajob's JSON API with page-based pagination.
func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	wanted := input.ResultsWanted
	if wanted <= 0 {
		wanted = 15
	}

	util.Debug("scraper_snagajob_start", map[string]any{
		"search_term":    input.SearchTerm,
		"location":       input.Location,
		"results_wanted": wanted,
	})

	jobs := make([]model.JobPost, 0, wanted)
	page := 0
	maxPages := (wanted / pageSize) + 1
	retries := 0

	for len(jobs) < wanted && page < maxPages && retries < maxRetries {
		select {
		case <-ctx.Done():
			util.Debug("scraper_snagajob_cancelled", map[string]any{"jobs_found": len(jobs)})
			return jobs, ctx.Err()
		default:
		}

		if page > 0 {
			if err := util.JitterSleep(ctx, rateLimitDelayMin, rateLimitDelayMax-rateLimitDelayMin); err != nil {
				return nil, err
			}
		}

		body, err := s.fetchPage(ctx, input.SearchTerm, input.Location, page)
		if err != nil {
			retries++
			util.Warn("scraper_snagajob_fetch_error", map[string]any{"page": page, "err": err.Error()})
			if retries < maxRetries {
				backoff(retries)
			}
			continue
		}

		pageJobs, err := parseJobs(body)
		if err != nil {
			retries++
			util.Warn("scraper_snagajob_parse_error", map[string]any{"page": page, "err": err.Error()})
			if retries < maxRetries {
				backoff(retries)
			}
			continue
		}

		if len(pageJobs) == 0 {
			util.Debug("scraper_snagajob_no_more", map[string]any{"page": page})
			break
		}

		for _, j := range pageJobs {
			jobs = append(jobs, j)
			if len(jobs) >= wanted {
				break
			}
		}

		page++
		retries = 0
	}

	if !util.HasMeaningfulJobs(jobs) {
		return nil, fmt.Errorf("snagajob no parseable jobs")
	}
	if len(jobs) > wanted {
		jobs = jobs[:wanted]
	}

	util.Debug("scraper_snagajob_done", map[string]any{"jobs": len(jobs)})
	return jobs, nil
}

// fetchPage downloads the JSON response for a given search/page.
func (s *Scraper) fetchPage(ctx context.Context, searchTerm, location string, page int) ([]byte, error) {
	u, err := url.Parse(s.searchURL)
	if err != nil {
		return nil, fmt.Errorf("snagajob parse url: %w", err)
	}
	q := u.Query()
	if v := strings.TrimSpace(searchTerm); v != "" {
		q.Set("q", v)
	}
	if v := strings.TrimSpace(location); v != "" {
		q.Set("location", v)
	}
	q.Set("radius", "25")
	q.Set("page", fmt.Sprintf("%d", page))
	q.Set("pageSize", fmt.Sprintf("%d", pageSize))
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("snagajob create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("snagajob request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("snagajob status %d — try using --proxy with a residential proxy", resp.StatusCode)
	}

	body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("snagajob read: %w", err)
	}
	return body, nil
}

// --- JSON response types ---

// apiResponse is the top-level JSON response from the Snagajob API.
// The jobs array can appear under "jobs", "results", or "data".
type apiResponse struct {
	Jobs    []rawJob `json:"jobs,omitempty"`
	Results []rawJob `json:"results,omitempty"`
	Data    []rawJob `json:"data,omitempty"`
}

// rawJob represents a single job listing from the Snagajob API.
type rawJob struct {
	ID          any     `json:"id,omitempty"`
	JobID       any     `json:"jobId,omitempty"`
	Title       string  `json:"title,omitempty"`
	JobTitle    string  `json:"jobTitle,omitempty"`
	URL         string  `json:"url,omitempty"`
	DetailURL   string  `json:"detailUrl,omitempty"`
	ApplyURL    string  `json:"applyUrl,omitempty"`
	Location    string  `json:"location,omitempty"`
	City        string  `json:"city,omitempty"`
	State       string  `json:"state,omitempty"`
	Company     string  `json:"company,omitempty"`
	CompanyName string  `json:"companyName,omitempty"`
	Employer    string  `json:"employer,omitempty"`
	PayMin      float64 `json:"payMin,omitempty"`
	MinPay      float64 `json:"minPay,omitempty"`
	SalaryMin   float64 `json:"salaryMin,omitempty"`
	PayMax      float64 `json:"payMax,omitempty"`
	MaxPay      float64 `json:"maxPay,omitempty"`
	SalaryMax   float64 `json:"salaryMax,omitempty"`
	Description string  `json:"description,omitempty"`
	Snippet     string  `json:"snippet,omitempty"`
	PostedDate  string  `json:"postedDate,omitempty"`
	DatePosted  string  `json:"datePosted,omitempty"`
	JobType     string  `json:"jobType,omitempty"`
}

// parseJobs extracts jobs from the Snagajob API JSON response.
func parseJobs(raw []byte) ([]model.JobPost, error) {
	var resp apiResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("snagajob unmarshal: %w", err)
	}

	var raws []rawJob
	switch {
	case len(resp.Jobs) > 0:
		raws = resp.Jobs
	case len(resp.Results) > 0:
		raws = resp.Results
	case len(resp.Data) > 0:
		raws = resp.Data
	default:
		return nil, nil
	}

	jobs := make([]model.JobPost, 0, len(raws))
	for _, r := range raws {
		job := r.toJobPost()
		if job == nil {
			continue
		}
		jobs = append(jobs, *job)
	}

	return jobs, nil
}

// toJobPost converts a raw API job into a model.JobPost.
func (r *rawJob) toJobPost() *model.JobPost {
	title := strings.TrimSpace(r.Title)
	if title == "" {
		title = strings.TrimSpace(r.JobTitle)
	}
	if title == "" {
		return nil
	}

	jobURL := strings.TrimSpace(r.URL)
	if jobURL == "" {
		jobURL = strings.TrimSpace(r.DetailURL)
	}
	if jobURL == "" {
		jobURL = strings.TrimSpace(r.ApplyURL)
	}
	if jobURL != "" && !strings.HasPrefix(jobURL, "http") {
		jobURL = "https://www.snagajob.com" + jobURL
	}

	companyName := strings.TrimSpace(r.Company)
	if companyName == "" {
		companyName = strings.TrimSpace(r.CompanyName)
	}
	if companyName == "" {
		companyName = strings.TrimSpace(r.Employer)
	}

	// --- Build location ---
	loc := model.Location{}
	locCity := strings.TrimSpace(r.City)
	if locCity == "" && r.Location != "" {
		locCity = strings.TrimSpace(r.Location)
	}
	loc.City = locCity
	loc.State = strings.TrimSpace(r.State)

	// --- Build compensation ---
	var comp *model.Compensation
	payMin := r.PayMin
	if payMin == 0 {
		payMin = r.MinPay
	}
	if payMin == 0 {
		payMin = r.SalaryMin
	}
	payMax := r.PayMax
	if payMax == 0 {
		payMax = r.MaxPay
	}
	if payMax == 0 {
		payMax = r.SalaryMax
	}
	if payMin > 0 || payMax > 0 {
		interval := model.IntervalHourly
		minAmt := payMin
		maxAmt := payMax
		comp = &model.Compensation{
			Interval:  interval,
			MinAmount: &minAmt,
			MaxAmount: &maxAmt,
			Currency:  "USD",
		}
	}

	// --- Build description ---
	desc := strings.TrimSpace(r.Description)
	if desc == "" {
		desc = strings.TrimSpace(r.Snippet)
	}
	if len(desc) > 500 {
		desc = desc[:500]
	}

	// --- Parse date ---
	var datePosted *time.Time
	dateStr := strings.TrimSpace(r.PostedDate)
	if dateStr == "" {
		dateStr = strings.TrimSpace(r.DatePosted)
	}
	if dateStr != "" {
		datePosted = util.ParseDatePosted(dateStr)
	}

	// --- Build ID ---
	jobID := fmt.Sprintf("snagajob-%v", r.ID)
	if r.ID == nil {
		jobID = fmt.Sprintf("snagajob-%v", r.JobID)
	}
	if r.ID == nil && r.JobID == nil {
		jobID = "snagajob-" + util.HashID(jobURL)
	}

	return &model.JobPost{
		ID:          jobID,
		Title:       title,
		CompanyName: companyName,
		JobURL:      jobURL,
		Location:    loc,
		Description: desc,
		Compensation: comp,
		DatePosted:  datePosted,
		JobType:     strings.ToLower(strings.TrimSpace(r.JobType)),
	}
}

func backoff(retries int) {
	if retries <= 0 {
		return
	}
	d := time.Duration(retries) * time.Second
	time.Sleep(d)
}


