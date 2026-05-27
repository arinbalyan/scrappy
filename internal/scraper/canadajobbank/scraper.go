package canadajobbank

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/util"
)

const (
	apiURL     = "https://jobbank.api.canada.ca/api/job"
	resourceID = "jobbank"
)

// canadaJobBankResponse wraps the API response.
type canadaJobBankResponse struct {
	Success bool                  `json:"success"`
	Result  *canadaJobBankResult  `json:"result,omitempty"`
}

type canadaJobBankResult struct {
	Records    []canadaJobBankRecord `json:"records"`
	Total      int                   `json:"total"`
}

type canadaJobBankRecord struct {
	ID              string  `json:"_id"`
	JobTitle        string  `json:"Job Title"`
	OriginalTitle   string  `json:"Original Job Title"`
	Company         string  `json:"Company"`
	City            string  `json:"City"`
	Province        string  `json:"Province/Territory"`
	SalaryMin       float64 `json:"Salary Minimum,omitempty"`
	SalaryMax       float64 `json:"Salary Maximum,omitempty"`
	SalaryPer       string  `json:"Salary Per,omitempty"`
	FirstPostingDate string `json:"First Posting Date"`
	NOCName         string  `json:"NOC21 Code Name"`
	EmploymentType  string  `json:"Employment Type"`
	EmploymentTerm  string  `json:"Employment Term"`
	Education       string  `json:"Education LOS"`
	Experience      string  `json:"Experience Level"`
	VacancyCount    int     `json:"Vacancy Count,omitempty"`
}

// Scraper fetches jobs from the Canada Job Bank API.
type Scraper struct {
	client  *http.Client
	apiURL  string
}

// New creates a new Canada Job Bank scraper.
func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Timeout: 20 * time.Second})
	}
	return &Scraper{client: client, apiURL: apiURL}
}

// NewWithAPIURL creates a scraper with a custom API URL (used in tests).
func NewWithAPIURL(client *http.Client, endpoint string) *Scraper {
	s := New(client)
	if strings.TrimSpace(endpoint) != "" {
		s.apiURL = endpoint
	}
	return s
}

// SiteName returns the site identifier.
func (s *Scraper) SiteName() model.Site { return model.SiteCanadaJobBank }

// Scrape fetches jobs from the Canada Job Bank API.
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
	if wanted > 500 {
		wanted = 500
	}

	queryParams := fmt.Sprintf("resource_id=%s&limit=%d", resourceID, wanted)
	if term := strings.TrimSpace(input.SearchTerm); term != "" {
		queryParams += "&q=" + strings.ReplaceAll(term, " ", "+")
	}

	url := s.apiURL + "?" + queryParams

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("canadajobbank: build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("canadajobbank: request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("canadajobbank: status %d", resp.StatusCode)
	}

	body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("canadajobbank: read: %w", err)
	}

	var apiResp canadaJobBankResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("canadajobbank: decode: %w", err)
	}

	if !apiResp.Success {
		return nil, fmt.Errorf("canadajobbank: API returned unsuccessful")
	}

	if apiResp.Result == nil {
		return nil, fmt.Errorf("canadajobbank: no result in response")
	}

	records := apiResp.Result.Records
	if len(records) == 0 {
		return nil, fmt.Errorf("canadajobbank: no jobs in response")
	}

	limit := wanted
	if limit > len(records) {
		limit = len(records)
	}

	out := make([]model.JobPost, 0, limit)
	for i := 0; i < limit; i++ {
		r := records[i]
		title := r.JobTitle
		if title == "" {
			title = r.OriginalTitle
		}
		if title == "" {
			continue
		}

		jobURL := fmt.Sprintf("https://www.jobbank.gc.ca/jobsearch/jobposting/%s", r.ID)

		// Build description from available fields
		var descParts []string
		if r.NOCName != "" {
			descParts = append(descParts, "Occupation: "+r.NOCName)
		}
		if r.EmploymentType != "" {
			descParts = append(descParts, "Type: "+r.EmploymentType)
		}
		if r.EmploymentTerm != "" {
			descParts = append(descParts, "Term: "+r.EmploymentTerm)
		}
		if r.Education != "" {
			descParts = append(descParts, "Education: "+r.Education)
		}
		if r.Experience != "" {
			descParts = append(descParts, "Experience: "+r.Experience)
		}
		if r.VacancyCount > 0 {
			descParts = append(descParts, fmt.Sprintf("Vacancies: %d", r.VacancyCount))
		}

		var compensation *model.Compensation
		if r.SalaryMin > 0 || r.SalaryMax > 0 {
			interval := model.IntervalYearly
			salaryPer := strings.ToLower(r.SalaryPer)
			if strings.Contains(salaryPer, "hour") {
				interval = model.IntervalHourly
			}
			compensation = &model.Compensation{
				Interval: interval,
				Currency: "CAD",
			}
			if r.SalaryMin > 0 {
				compensation.MinAmount = &r.SalaryMin
			}
			if r.SalaryMax > 0 {
				compensation.MaxAmount = &r.SalaryMax
			}
		}

		location := model.Location{
			City:    strings.TrimSpace(r.City),
			State:   strings.TrimSpace(r.Province),
			Country: "Canada",
		}

		var datePosted *time.Time
		if dp := strings.TrimSpace(r.FirstPostingDate); dp != "" {
			datePosted = util.ParseDatePosted(dp)
		}

		job := model.JobPost{
			ID:           "canadajobbank-" + r.ID,
			Title:        title,
			JobURL:       jobURL,
			Location:     location,
			Description:  strings.Join(descParts, "\n"),
			Compensation: compensation,
			DatePosted:   datePosted,
			Site:         string(s.SiteName()),
		}
		out = append(out, job)
	}

	util.Debug("scraper_done", map[string]any{
		"site": s.SiteName(),
		"jobs": len(out),
	})

	if !util.HasMeaningfulJobs(out) {
		return nil, fmt.Errorf("canadajobbank: no parseable jobs")
	}
	return out, nil
}
