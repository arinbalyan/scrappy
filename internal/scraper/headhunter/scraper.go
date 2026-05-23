package headhunter

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
	searchURL       = "https://api.hh.ru/vacancies"
	defaultResults  = 25
	rateLimitDelay  = 333 * time.Millisecond // ~3 req/s
	maxRetries      = 3
)

// Scraper scrapes HeadHunter (hh.ru) job listings via their public API.
type Scraper struct {
	client    *http.Client
	searchURL string
}

// New creates a new HeadHunter scraper with the given HTTP client.
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
func (s *Scraper) SiteName() model.Site { return model.SiteHeadHunter }

// Scrape fetches job listings from the HeadHunter API.
func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	wanted := input.ResultsWanted
	if wanted <= 0 {
		wanted = defaultResults
	}
	util.Debug("scraper_headhunter_start", map[string]any{
		"search_term":    input.SearchTerm,
		"location":       input.Location,
		"results_wanted": wanted,
	})

	body, err := s.fetchVacancies(ctx, input.SearchTerm, wanted)
	if err != nil {
		return nil, fmt.Errorf("headhunter fetch: %w", err)
	}

	var resp apiResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("headhunter unmarshal: %w", err)
	}

	if len(resp.Items) == 0 {
		return nil, fmt.Errorf("headhunter no vacancies found")
	}

	jobs := make([]model.JobPost, 0, min(wanted, len(resp.Items)))
	for _, v := range resp.Items {
		if len(jobs) >= wanted {
			break
		}
		job := mapVacancy(v)
		if job != nil {
			jobs = append(jobs, *job)
		}
	}

	if !util.HasMeaningfulJobs(jobs) {
		return nil, fmt.Errorf("headhunter no parseable jobs")
	}
	if len(jobs) > wanted {
		jobs = jobs[:wanted]
	}
	util.Debug("scraper_headhunter_done", map[string]any{"jobs": len(jobs)})
	return jobs, nil
}

// fetchVacancies calls the HeadHunter API and returns the raw JSON body.
func (s *Scraper) fetchVacancies(ctx context.Context, searchTerm string, perPage int) ([]byte, error) {
	u, _ := url.Parse(s.searchURL)
	q := u.Query()
	if v := strings.TrimSpace(searchTerm); v != "" {
		q.Set("text", v)
	}
	q.Set("per_page", fmt.Sprintf("%d", perPage))
	q.Set("page", "0")
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "scrappy/0.1.0 (job-aggregator)")

	time.Sleep(rateLimitDelay)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("headhunter request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("headhunter status %d", resp.StatusCode)
	}

	body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("headhunter read: %w", err)
	}
	return body, nil
}

// mapVacancy converts a HeadHunter API vacancy to a JobPost.
func mapVacancy(v vacancy) *model.JobPost {
	if strings.TrimSpace(v.Name) == "" || strings.TrimSpace(v.ID) == "" {
		return nil
	}

	location := model.Location{
		Country: "Russia",
	}
	if v.Area != nil && strings.TrimSpace(v.Area.Name) != "" {
		location.City = strings.TrimSpace(v.Area.Name)
	}

	var comp *model.Compensation
	if v.Salary != nil && (v.Salary.From != nil || v.Salary.To != nil) {
		currency := "RUR"
		if v.Salary.Currency != "" {
			currency = v.Salary.Currency
		}
		comp = &model.Compensation{
			Interval: model.IntervalMonthly,
			Currency: currency,
		}
		if v.Salary.From != nil {
			val := float64(*v.Salary.From)
			comp.MinAmount = &val
		}
		if v.Salary.To != nil {
			val := float64(*v.Salary.To)
			comp.MaxAmount = &val
		}
	}

	description := buildDescription(v.Snippet)
	jobURL := ""
	if v.AlternateURL != nil {
		jobURL = *v.AlternateURL
	}

	var datePosted *time.Time
	if v.PublishedAt != nil {
		if t, err := time.Parse(time.RFC3339, *v.PublishedAt); err == nil {
			datePosted = &t
		}
	}

	isRemote := false
	if v.Schedule != nil && v.Schedule.ID != nil && *v.Schedule.ID == "remote" {
		isRemote = true
	}
	if v.WorkFormat != nil {
		for _, wf := range v.WorkFormat {
			if wf.ID != nil && strings.ToUpper(*wf.ID) == "REMOTE" {
				isRemote = true
				break
			}
		}
	}

	companyName := ""
	if v.Employer != nil && v.Employer.Name != nil {
		companyName = *v.Employer.Name
	}

	return &model.JobPost{
		ID:          fmt.Sprintf("headhunter-%s", v.ID),
		Title:       strings.TrimSpace(v.Name),
		CompanyName: companyName,
		JobURL:      jobURL,
		Location:    location,
		Description: description,
		Compensation: comp,
		DatePosted:  datePosted,
		IsRemote:    isRemote,
	}
}

// buildDescription constructs a description from the snippet fields.
func buildDescription(s *snippet) string {
	if s == nil {
		return ""
	}
	parts := make([]string, 0, 2)
	if s.Requirement != nil && strings.TrimSpace(*s.Requirement) != "" {
		parts = append(parts, "Requirements: "+strings.TrimSpace(*s.Requirement))
	}
	if s.Responsibility != nil && strings.TrimSpace(*s.Responsibility) != "" {
		parts = append(parts, "Responsibilities: "+strings.TrimSpace(*s.Responsibility))
	}
	return strings.Join(parts, "\n")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// --- JSON types for the HeadHunter API ---

type apiResponse struct {
	Items   []vacancy `json:"items"`
	Found   int       `json:"found"`
	Pages   int       `json:"pages"`
	PerPage int       `json:"per_page"`
}

type vacancy struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Area         *area     `json:"area,omitempty"`
	Salary       *salary   `json:"salary,omitempty"`
	Employer     *employer `json:"employer,omitempty"`
	Snippet      *snippet  `json:"snippet,omitempty"`
	AlternateURL *string   `json:"alternate_url,omitempty"`
	PublishedAt  *string   `json:"published_at,omitempty"`
	Schedule     *schedule `json:"schedule,omitempty"`
	WorkFormat   []wfItem  `json:"work_format,omitempty"`
}

type area struct {
	Name string `json:"name"`
}

type salary struct {
	From     *int    `json:"from,omitempty"`
	To       *int    `json:"to,omitempty"`
	Currency string `json:"currency,omitempty"`
	Gross    *bool   `json:"gross,omitempty"`
}

type employer struct {
	Name *string `json:"name,omitempty"`
}

type snippet struct {
	Requirement    *string `json:"requirement,omitempty"`
	Responsibility *string `json:"responsibility,omitempty"`
}

type schedule struct {
	ID *string `json:"id,omitempty"`
}

type wfItem struct {
	ID *string `json:"id,omitempty"`
}
