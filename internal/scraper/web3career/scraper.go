package web3career

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

const defaultAPI = "https://web3.career/api/v1"

// rawJob mirrors the Web3Career API job object (returned as array or wrapped in {data,jobs}).
type rawJob struct {
	ID             interface{} `json:"id"`
	Title          string      `json:"title"`
	Company        string      `json:"company"`
	CompanyLogo    string      `json:"company_logo"`
	URL            string      `json:"url"`
	Link           string      `json:"link"`
	Description    string      `json:"description"`
	Location       string      `json:"location"`
	Tags           []string    `json:"tags"`
	Category       string      `json:"category"`
	SalaryMin      float64     `json:"salary_min"`
	SalaryMax      float64     `json:"salary_max"`
	SalaryCurrency string      `json:"salary_currency"`
	DatePosted     string      `json:"date_posted"`
	CreatedAt      string      `json:"created_at"`
	IsRemote       *bool       `json:"is_remote"`
	Remote         *bool       `json:"remote"`
}

// rawResponse handles the case where the API returns {data: [...]} or {jobs: [...]}.
type rawResponse struct {
	Data []rawJob `json:"data"`
	Jobs []rawJob `json:"jobs"`
}

// Scraper for web3.career.
type Scraper struct {
	client *http.Client
	apiURL string
}

// New creates a Web3Career scraper with the given HTTP client.
func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Retries: 2, CookieResetEveryN: 120, Timeout: 15 * time.Second})
	}
	return &Scraper{client: client, apiURL: defaultAPI}
}

// NewWithAPIURL creates a scraper pointing at a custom endpoint (used in tests).
func NewWithAPIURL(client *http.Client, endpoint string) *Scraper {
	s := New(client)
	if strings.TrimSpace(endpoint) != "" {
		s.apiURL = endpoint
	}
	return s
}

// SiteName returns the site identifier.
func (s *Scraper) SiteName() model.Site { return model.SiteWeb3Career }

// Scrape fetches jobs from web3.career, optionally filtering by search term.
func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{
		"site":           s.SiteName(),
		"results_wanted": input.ResultsWanted,
		"search_term":    input.SearchTerm,
	})

	u, _ := url.Parse(s.apiURL)
	q := u.Query()
	q.Set("token", "public")
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("web3career request create: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/129.0.0.0 Safari/537.36")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("web3career request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("web3career status %d", resp.StatusCode)
	}

	body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("web3career read: %w", err)
	}

	entries, err := parseResponse(body)
	if err != nil {
		return nil, fmt.Errorf("web3career parse: %w", err)
	}

	// Filter by search term if provided (API returns all jobs; client-side filtering).
	term := strings.ToLower(strings.TrimSpace(input.SearchTerm))
	if term != "" {
		filtered := make([]rawJob, 0, len(entries))
		for _, j := range entries {
			if strings.Contains(strings.ToLower(j.Title), term) || tagContains(j.Tags, term) {
				filtered = append(filtered, j)
			}
		}
		entries = filtered
	}

	limit := input.ResultsWanted
	if limit <= 0 || limit > len(entries) {
		limit = len(entries)
	}
	jobs := make([]model.JobPost, 0, limit)
	for i := 0; i < limit; i++ {
		j := entries[i]
		job, ok := mapJob(j)
		if ok {
			jobs = append(jobs, job)
		}
	}

	util.Debug("scraper_done", map[string]any{"site": s.SiteName(), "jobs": len(jobs)})
	if !util.HasMeaningfulJobs(jobs) {
		return nil, fmt.Errorf("web3career no parseable jobs")
	}
	return jobs, nil
}

func parseResponse(body []byte) ([]rawJob, error) {
	// Try direct array first.
	var arr []rawJob
	if err := json.Unmarshal(body, &arr); err == nil {
		return arr, nil
	}

	// Try object wrapper {data: [...]} or {jobs: [...]}.
	var resp rawResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	if len(resp.Data) > 0 {
		return resp.Data, nil
	}
	if len(resp.Jobs) > 0 {
		return resp.Jobs, nil
	}
	return nil, nil
}

func mapJob(j rawJob) (model.JobPost, bool) {
	jobURL := j.URL
	if jobURL == "" {
		jobURL = j.Link
	}
	if j.Title == "" || jobURL == "" {
		return model.JobPost{}, false
	}

	// Build id from the raw id field.
	idStr := fmt.Sprintf("%v", j.ID)
	id := "w3c-" + idStr

	// Build location.
	loc := model.Location{City: j.Location}

	// Build compensation.
	var comp *model.Compensation
	if j.SalaryMin > 0 && j.SalaryMax > 0 {
		minVal := j.SalaryMin
		maxVal := j.SalaryMax
		currency := j.SalaryCurrency
		if currency == "" {
			currency = "USD"
		}
		comp = &model.Compensation{
			Interval:  model.IntervalYearly,
			MinAmount: &minVal,
			MaxAmount: &maxVal,
			Currency:  currency,
		}
	}

	// Parse date.
	var posted *time.Time
	rawDate := j.DatePosted
	if rawDate == "" {
		rawDate = j.CreatedAt
	}
	if rawDate != "" {
		if t, err := time.Parse("2006-01-02", rawDate[:10]); err == nil {
			posted = &t
		}
	}

	// Determine remote status.
	remote := false
	if j.IsRemote != nil {
		remote = *j.IsRemote
	} else if j.Remote != nil {
		remote = *j.Remote
	}

	return model.JobPost{
		ID:             id,
		Title:          j.Title,
		CompanyName:    j.Company,
		CompanyLogoURL: j.CompanyLogo,
		JobURL:         jobURL,
		Location:       loc,
		Description:    j.Description,
		IsRemote:       remote,
		DatePosted:     posted,
		Compensation:   comp,
		Skills:         j.Tags,
		Department:     j.Category,
	}, true
}

func tagContains(tags []string, term string) bool {
	for _, tag := range tags {
		if strings.Contains(strings.ToLower(tag), term) {
			return true
		}
	}
	return false
}
