package habrcareer

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

const apiURL = "https://career.habr.com/api/frontend/vacancies"

// Scraper fetches jobs from the Habr Career API.
type Scraper struct {
	client *http.Client
	apiURL string
}

// New creates a new Habr Career scraper. If client is nil a default one is used.
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
func (s *Scraper) SiteName() model.Site { return model.SiteHabrCareer }

// Scrape fetches jobs from the Habr Career API.
// API: GET /api/frontend/vacancies?page=1&per_page=N&type=all&sort=date&q=...
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

	var allJobs []model.JobPost

	// Habr Career supports pagination; fetch up to 5 pages
	for page := 1; page <= 5; page++ {
		select {
		case <-ctx.Done():
			return allJobs, ctx.Err()
		default:
		}

		if len(allJobs) >= wanted {
			break
		}

		url := fmt.Sprintf("%s?page=%d&per_page=%d&type=all&sort=date", s.apiURL, page, wanted)
		if input.SearchTerm != "" {
			url += "&q=" + input.SearchTerm
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("habrcareer: build request: %w", err)
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "Mozilla/5.0")

		resp, err := s.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("habrcareer: request: %w", err)
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			resp.Body.Close()
			return nil, fmt.Errorf("habrcareer: status %d", resp.StatusCode)
		}

		body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("habrcareer: read: %w", err)
		}

		var apiResp habrcareerAPIResponse
		if err := json.Unmarshal(body, &apiResp); err != nil {
			return nil, fmt.Errorf("habrcareer: decode: %w", err)
		}

		if len(apiResp.List) == 0 {
			break
		}

		remaining := wanted - len(allJobs)
		for _, raw := range apiResp.List {
			if remaining <= 0 {
				break
			}
			job := mapJob(raw)
			if job != nil {
				allJobs = append(allJobs, *job)
				remaining--
			}
		}

		// If we got fewer items than requested, stop paginating
		if len(apiResp.List) < wanted {
			break
		}
	}

	util.Debug("scraper_done", map[string]any{
		"site": s.SiteName(),
		"jobs": len(allJobs),
	})

	if !util.HasMeaningfulJobs(allJobs) {
		return nil, fmt.Errorf("habrcareer: no parseable jobs")
	}
	return allJobs, nil
}

// habrcareerAPIResponse maps the API JSON response.
type habrcareerAPIResponse struct {
	List []vacancy `json:"list"`
	Meta *meta     `json:"meta,omitempty"`
}

type meta struct {
	TotalResults int `json:"totalResults"`
}

// vacancy mirrors the Habr Career vacancy object.
type vacancy struct {
	ID                 int                `json:"id"`
	Href               string             `json:"href,omitempty"`
	Title              string             `json:"title"`
	RemoteWork         bool               `json:"remoteWork,omitempty"`
	SalaryQualification *salaryQual       `json:"salaryQualification,omitempty"`
	PublishedDate      *publishedDate     `json:"publishedDate,omitempty"`
	Company            *company           `json:"company,omitempty"`
	Employment         string             `json:"employment,omitempty"`
	Salary             *salary            `json:"salary,omitempty"`
	Divisions          []division         `json:"divisions,omitempty"`
	Skills             []skill            `json:"skills,omitempty"`
	Locations          []location         `json:"locations,omitempty"`
}

type salaryQual struct {
	Title string `json:"title,omitempty"`
}

type publishedDate struct {
	Date string `json:"date,omitempty"`
}

type company struct {
	Title string `json:"title,omitempty"`
	Href  string `json:"href,omitempty"`
}

type salary struct {
	From      float64 `json:"from,omitempty"`
	To        float64 `json:"to,omitempty"`
	Currency  string  `json:"currency,omitempty"`
	Formatted string  `json:"formatted,omitempty"`
}

type division struct {
	Title string `json:"title,omitempty"`
}

type skill struct {
	Title string `json:"title,omitempty"`
}

type location struct {
	Title string `json:"title,omitempty"`
}

// currencyMap maps Habr Career currency codes to ISO codes.
var currencyMap = map[string]string{
	"rur": "RUB",
	"usd": "USD",
	"eur": "EUR",
	"kzt": "KZT",
	"uah": "UAH",
	"gbp": "GBP",
}

// employmentMap maps Habr Career employment types to standard job types.
var employmentMap = map[string]string{
	"full_time":  "fulltime",
	"part_time":  "parttime",
	"contract":   "contract",
	"internship": "internship",
	"volunteer":  "volunteer",
}

// mapJob converts a raw vacancy into a JobPost.
func mapJob(raw vacancy) *model.JobPost {
	title := strings.TrimSpace(raw.Title)
	if title == "" {
		return nil
	}

	jobURL := ""
	if raw.Href != "" {
		jobURL = "https://career.habr.com" + raw.Href
	}
	if jobURL == "" {
		return nil
	}

	// Build description from divisions, skills, employment, qualification
	var descParts []string
	if len(raw.Divisions) > 0 {
		var titles []string
		for _, d := range raw.Divisions {
			if strings.TrimSpace(d.Title) != "" {
				titles = append(titles, strings.TrimSpace(d.Title))
			}
		}
		if len(titles) > 0 {
			descParts = append(descParts, "Role: "+strings.Join(titles, ", "))
		}
	}
	if len(raw.Skills) > 0 {
		var titles []string
		for _, s := range raw.Skills {
			if strings.TrimSpace(s.Title) != "" {
				titles = append(titles, strings.TrimSpace(s.Title))
			}
		}
		if len(titles) > 0 {
			descParts = append(descParts, "Skills: "+strings.Join(titles, ", "))
		}
	}
	if raw.Employment != "" {
		descParts = append(descParts, "Employment: "+raw.Employment)
	}
	if raw.SalaryQualification != nil && strings.TrimSpace(raw.SalaryQualification.Title) != "" {
		descParts = append(descParts, "Level: "+strings.TrimSpace(raw.SalaryQualification.Title))
	}

	description := ""
	if len(descParts) > 0 {
		description = strings.Join(descParts, "\n")
	}

	// Build location
	loc := model.Location{}
	if len(raw.Locations) > 0 {
		loc.City = strings.TrimSpace(raw.Locations[0].Title)
	}

	// Build compensation
	var compensation *model.Compensation
	if raw.Salary != nil {
		hasFrom := raw.Salary.From > 0
		hasTo := raw.Salary.To > 0
		if hasFrom || hasTo {
			rawCurrency := strings.ToLower(raw.Salary.Currency)
			if rawCurrency == "" {
				rawCurrency = "rur"
			}
			currency := currencyMap[rawCurrency]
			if currency == "" {
				currency = strings.ToUpper(rawCurrency)
			}
			compensation = &model.Compensation{
				Interval: model.IntervalMonthly,
				Currency: currency,
			}
			if hasFrom {
				minVal := raw.Salary.From
				compensation.MinAmount = &minVal
			}
			if hasTo {
				maxVal := raw.Salary.To
				compensation.MaxAmount = &maxVal
			}
		}
	}

	// Parse date
	var datePosted *time.Time
	if raw.PublishedDate != nil && raw.PublishedDate.Date != "" {
		datePosted = util.ParseDatePosted(raw.PublishedDate.Date)
	}

	// Map employment type
	jobType := ""
	if raw.Employment != "" {
		normalized := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(raw.Employment, " ", "_"), "-", "_"))
		if mapped, ok := employmentMap[normalized]; ok {
			jobType = mapped
		}
	}

	return &model.JobPost{
		ID:          fmt.Sprintf("habrcareer-%d", raw.ID),
		Title:       title,
		CompanyName: strings.TrimSpace(raw.Company.Title),
		JobURL:      jobURL,
		Description: description,
		Location:    loc,
		IsRemote:    raw.RemoteWork,
		Compensation: compensation,
		DatePosted:  datePosted,
		JobType:     jobType,
		Site:        string(model.SiteHabrCareer),
		ApplyMethod: "external_url",
	}
}
