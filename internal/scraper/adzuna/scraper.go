package adzuna

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/util"
)

const (
	defaultAPIBase  = "https://api.adzuna.com/v1/api/jobs"
	defaultPageSize = 50
	maxPages        = 20
)

var (
	stripTagsRe = regexp.MustCompile(`(?is)<[^>]+>`)

	// countryCodes maps model.Country to Adzuna's 2-letter country codes.
	countryCodes = map[model.Country]string{
		model.CountryUSA:       "us",
		model.CountryCanada:    "ca",
		model.CountryUK:        "gb",
		model.CountryGermany:   "de",
		model.CountryFrance:    "fr",
		model.CountryIndia:     "in",
		model.CountryAustralia: "au",
	}

	// countryCurrencies maps model.Country to default currency for salary.
	countryCurrencies = map[model.Country]string{
		model.CountryUSA:       "USD",
		model.CountryCanada:    "CAD",
		model.CountryUK:        "GBP",
		model.CountryGermany:   "EUR",
		model.CountryFrance:    "EUR",
		model.CountryIndia:     "INR",
		model.CountryAustralia: "AUD",
	}

	// contractTypeMap translates Adzuna contract_time values to our JobType strings.
	contractTypeMap = map[string]string{
		"full_time": "fulltime",
		"part_time": "parttime",
		"contract":  "contract",
	}
)

// apiResponse is the top-level Adzuna API response.
type apiResponse struct {
	Count   int      `json:"count"`
	Results []apiJob `json:"results"`
}

// apiJob is a single job posting from Adzuna.
type apiJob struct {
	ID                string    `json:"id"`
	Title             string    `json:"title"`
	Company           apiCompany `json:"company"`
	RedirectURL       string    `json:"redirect_url"`
	Location          apiLocation `json:"location"`
	SalaryMin         *float64  `json:"salary_min"`
	SalaryMax         *float64  `json:"salary_max"`
	SalaryIsPredicted string    `json:"salary_is_predicted"`
	Created           string    `json:"created"`
	ContractTime      string    `json:"contract_time"`
	Description       string    `json:"description"`
	Adref             string    `json:"adref"`
}

type apiCompany struct {
	DisplayName string `json:"display_name"`
}

type apiLocation struct {
	DisplayName string `json:"display_name"`
}

// Scraper scrapes jobs from the Adzuna API.
type Scraper struct {
	client  *http.Client
	apiBase string
	appID   string
	appKey  string
}

// New creates an Adzuna scraper. Credentials are read from ADZUNA_APP_ID and
// ADZUNA_APP_KEY environment variables.
func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Retries: 2, Timeout: 20 * time.Second})
	}
	return &Scraper{
		client:  client,
		apiBase: defaultAPIBase,
		appID:   os.Getenv("ADZUNA_APP_ID"),
		appKey:  os.Getenv("ADZUNA_APP_KEY"),
	}
}

// NewWithURLs creates an Adzuna scraper with explicit configuration. Used for
// testing to point at a local test server.
func NewWithURLs(client *http.Client, apiBase, appID, appKey string) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Retries: 2, Timeout: 20 * time.Second})
	}
	return &Scraper{
		client:  client,
		apiBase: apiBase,
		appID:   appID,
		appKey:  appKey,
	}
}

// SiteName returns model.SiteAdzuna.
func (s *Scraper) SiteName() model.Site { return model.SiteAdzuna }

// Scrape fetches jobs from the Adzuna API. It handles page-based pagination,
// deduplicates by job ID, and maps the Adzuna response to model.JobPost.
func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	if s.appID == "" || s.appKey == "" {
		return nil, fmt.Errorf("adzuna missing credentials: set ADZUNA_APP_ID and ADZUNA_APP_KEY")
	}

	wanted := input.ResultsWanted
	if wanted <= 0 {
		wanted = 25
	}

	pageSize := defaultPageSize
	if wanted < pageSize {
		pageSize = wanted
	}

	cc := resolveCountryCode(input.Country)

	jobs := make([]model.JobPost, 0, wanted)
	seen := make(map[string]struct{})
	searchTerm := strings.TrimSpace(input.SearchTerm)
	location := strings.TrimSpace(input.Location)

	for page := 1; len(jobs) < wanted && page <= maxPages; page++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		u := fmt.Sprintf("%s/%s/search/%d", s.apiBase, cc, page)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return nil, fmt.Errorf("adzuna request: %w", err)
		}

		q := req.URL.Query()
		q.Set("app_id", s.appID)
		q.Set("app_key", s.appKey)
		q.Set("results_per_page", fmt.Sprintf("%d", pageSize))
		q.Set("sort_by", "date")

		if searchTerm != "" {
			q.Set("what", searchTerm)
		}
		if location != "" {
			q.Set("where", location)
		}
		if input.HoursOld > 0 {
			days := input.HoursOld/24 + 1
			if days < 1 {
				days = 1
			}
			q.Set("max_days_old", fmt.Sprintf("%d", days))
		}
		req.URL.RawQuery = q.Encode()

		req.Header.Set("Accept", "application/json")

		resp, err := s.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("adzuna request: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("adzuna status %d", resp.StatusCode)
		}

		body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
		if err != nil {
			return nil, fmt.Errorf("adzuna read: %w", err)
		}

		var parsed apiResponse
		if err := json.Unmarshal(body, &parsed); err != nil {
			return nil, fmt.Errorf("adzuna decode: %w", err)
		}

		if len(parsed.Results) == 0 {
			if len(jobs) == 0 {
				return nil, fmt.Errorf("adzuna no results")
			}
			break
		}

		currency := resolveCurrency(input.Country)

		for _, r := range parsed.Results {
			if len(jobs) >= wanted {
				break
			}

			title := strings.TrimSpace(r.Title)
			jobURL := strings.TrimSpace(r.RedirectURL)
			if title == "" || jobURL == "" {
				continue
			}

			id := "adzuna-" + strings.TrimSpace(r.ID)
			if _, exists := seen[id]; exists {
				continue
			}
			seen[id] = struct{}{}

			job := model.JobPost{
				ID:          id,
				Title:       title,
				CompanyName: strings.TrimSpace(r.Company.DisplayName),
				JobURL:      jobURL,
				Description: cleanHTML(r.Description),
				JobType:     mapContractType(r.ContractTime),
			}

			if loc := strings.TrimSpace(r.Location.DisplayName); loc != "" {
				job.Location = model.Location{City: loc}
			}

			// Salary: include only when not marked as predicted.
			if r.SalaryIsPredicted != "1" && (r.SalaryMin != nil || r.SalaryMax != nil) {
				job.Compensation = &model.Compensation{
					Interval:  model.IntervalYearly,
					MinAmount: r.SalaryMin,
					MaxAmount: r.SalaryMax,
					Currency:  currency,
				}
			}

			if v := strings.TrimSpace(r.Created); v != "" {
				if t, err := time.Parse(time.RFC3339, v); err == nil {
					t = t.UTC()
					job.DatePosted = &t
				}
			}

			jobs = append(jobs, job)
		}

		// Rate limit: 500ms between pages to stay well under Adzuna's 1-3 rps limit.
		if err := util.SleepWithContext(ctx, 500*time.Millisecond); err != nil {
			return nil, ctx.Err()
		}
	}

	if !util.HasMeaningfulJobs(jobs) {
		return nil, fmt.Errorf("adzuna no parseable jobs")
	}

	if len(jobs) > wanted {
		jobs = jobs[:wanted]
	}

	return jobs, nil
}

// resolveCountryCode maps our Country to an Adzuna 2-letter code. Defaults to "us".
func resolveCountryCode(c model.Country) string {
	if code, ok := countryCodes[c]; ok {
		return code
	}
	return "us"
}

// resolveCurrency returns the default currency for a country. Defaults to "USD".
func resolveCurrency(c model.Country) string {
	if cur, ok := countryCurrencies[c]; ok {
		return cur
	}
	return "USD"
}

// cleanHTML strips HTML tags from a string and normalises whitespace.
func cleanHTML(v string) string {
	if strings.TrimSpace(v) == "" {
		return ""
	}
	v = stripTagsRe.ReplaceAllString(v, " ")
	return strings.Join(strings.Fields(strings.TrimSpace(v)), " ")
}

// mapContractType translates Adzuna contract_time to our JobType string.
// Returns empty string if no match.
func mapContractType(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	if t, ok := contractTypeMap[strings.ToLower(v)]; ok {
		return t
	}
	return ""
}
