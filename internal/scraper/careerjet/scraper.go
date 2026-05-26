package careerjet

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/util"
)

const (
	apiURL         = "https://public.api.careerjet.net/search"
	maxPages       = 10
	defaultPageSize = 50
	minInterval    = 350 * time.Millisecond // ~3 req/s rate limit
)

// ─── API Response Types ─────────────────────────────────────────────────────────

// apiResponse is the top-level CareerJet API response.
type apiResponse struct {
	Type         string   `json:"type"`
	Hits         int      `json:"hits"`
	Pages        int      `json:"pages"`
	ResponseTime int      `json:"response_time"`
	Jobs         []apiJob `json:"jobs"`
}

// apiJob is a single job posting from the CareerJet API.
type apiJob struct {
	Title              string  `json:"title"`
	Company            string  `json:"company"`
	Date               string  `json:"date"`
	Description        string  `json:"description"`
	Locations          string  `json:"locations"`
	URL                string  `json:"url"`
	Site               string  `json:"site"`
	Salary             json.RawMessage `json:"salary"`
	SalaryMin          float64 `json:"salary_min"`
	SalaryMax          float64 `json:"salary_max"`
	SalaryType         string  `json:"salary_type"`          // Y=yearly, M=monthly, W=weekly, D=daily, H=hourly
	SalaryCurrencyCode string  `json:"salary_currency_code"`
}

// salaryTypeMap maps CareerJet salary interval codes to model intervals.
var salaryTypeMap = map[string]model.CompensationInterval{
	"Y": model.IntervalYearly,
	"M": model.IntervalMonthly,
	"W": model.IntervalWeekly,
	"D": model.IntervalDaily,
	"H": model.IntervalHourly,
}

// countryToLocale maps model.Country to CareerJet locale codes.
var countryToLocale = map[model.Country]string{
	model.CountryUSA:       "en_US",
	model.CountryCanada:    "en_CA",
	model.CountryUK:        "en_GB",
	model.CountryGermany:   "de_DE",
	model.CountryFrance:    "fr_FR",
	model.CountryIndia:     "en_IN",
	model.CountryAustralia: "en_AU",
}

// ─── Scraper ────────────────────────────────────────────────────────────────────

// Scraper scrapes jobs from the CareerJet public API.
type Scraper struct {
	client *http.Client
	apiURL string
	affID  string
}

// New creates a new CareerJet scraper. Credentials are read from the
// CAREERJET_AFFID environment variable. If client is nil, a default
// HTTP client with retries and timeout is created.
func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Retries: 2, Timeout: 25 * time.Second})
	}
	return &Scraper{
		client: client,
		apiURL: apiURL,
		affID:  os.Getenv("CAREERJET_AFFID"),
	}
}

// NewWithURLs creates a scraper with explicit configuration for testing.
func NewWithURLs(client *http.Client, apiURL, affID string) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Retries: 2, Timeout: 25 * time.Second})
	}
	s := &Scraper{client: client, apiURL: apiURL, affID: affID}
	if strings.TrimSpace(apiURL) != "" {
		s.apiURL = strings.TrimSpace(apiURL)
	}
	return s
}

// SiteName returns model.SiteCareerjet.
func (s *Scraper) SiteName() model.Site { return model.SiteCareerjet }

// Scrape fetches jobs from the CareerJet API. It handles page-based pagination,
// rate-limits to ~3 requests/second, deduplicates by job URL, and maps the
// CareerJet response to model.JobPost.
func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	if s.affID == "" {
		return nil, fmt.Errorf("careerjet: missing affiliate ID, set CAREERJET_AFFID")
	}

	wanted := input.ResultsWanted
	if wanted <= 0 {
		wanted = 15
	}

	searchTerm := strings.TrimSpace(input.SearchTerm)
	if searchTerm == "" {
		return nil, fmt.Errorf("careerjet: search term required")
	}

	locale := resolveLocale(input.Country)
	pageSize := defaultPageSize
	if wanted < pageSize {
		pageSize = wanted
	}

	// Rate limiter: ~3 requests per second (350ms minimum interval).
	rateLimiter := time.NewTicker(minInterval)
	defer rateLimiter.Stop()

	jobs := make([]model.JobPost, 0, wanted)
	seen := make(map[string]struct{})

	for page := 1; page <= maxPages && len(jobs) < wanted; page++ {
		select {
		case <-ctx.Done():
			return jobs, ctx.Err()
		default:
		}

		body, err := s.fetchPage(ctx, searchTerm, input.Location, locale, page, pageSize)
		if err != nil {
			return nil, fmt.Errorf("careerjet page %d: %w", page, err)
		}

		var parsed apiResponse
		if err := json.Unmarshal(body, &parsed); err != nil {
			return nil, fmt.Errorf("careerjet decode page %d: %w", page, err)
		}

		if len(parsed.Jobs) == 0 {
			break
		}

		for _, j := range parsed.Jobs {
			if len(jobs) >= wanted {
				break
			}

			title := strings.TrimSpace(j.Title)
			jobURL := strings.TrimSpace(j.URL)
			if title == "" || jobURL == "" {
				continue
			}

			if _, exists := seen[jobURL]; exists {
				continue
			}
			seen[jobURL] = struct{}{}

			job := model.JobPost{
				ID:          "cj-" + util.HashID(jobURL),
				Title:       title,
				CompanyName: strings.TrimSpace(j.Company),
				JobURL:      jobURL,
				Description: strings.TrimSpace(j.Description),
			}

			// Location: parse "City, State" format.
			if loc := strings.TrimSpace(j.Locations); loc != "" {
				job.Location = parseLocation(loc)
			}

			// Date posted.
			if v := strings.TrimSpace(j.Date); v != "" {
				job.DatePosted = util.ParseDatePosted(v)
			}

			// Compensation.
			if j.SalaryMin > 0 {
				comp := &model.Compensation{
					Interval: resolveSalaryInterval(j.SalaryType),
					Currency: j.SalaryCurrencyCode,
				}
				if comp.Currency == "" {
					comp.Currency = "USD"
				}
				minAmt := j.SalaryMin
				comp.MinAmount = &minAmt
				if j.SalaryMax > 0 {
					maxAmt := j.SalaryMax
					comp.MaxAmount = &maxAmt
				}
				job.Compensation = comp
			}

			jobs = append(jobs, job)
		}

		// Stop if the API returned fewer results than the page size
		// (indicating no more pages) or if we've hit the last page.
		if len(parsed.Jobs) < pageSize {
			break
		}
		if page >= parsed.Pages {
			break
		}

		// Rate-limit before the next request.
		select {
		case <-ctx.Done():
			return jobs, ctx.Err()
		case <-rateLimiter.C:
		}
	}

	if !util.HasMeaningfulJobs(jobs) {
		return nil, fmt.Errorf("careerjet no parseable jobs")
	}
	if len(jobs) > wanted {
		jobs = jobs[:wanted]
	}
	return jobs, nil
}

// ─── HTTP ───────────────────────────────────────────────────────────────────────

// fetchPage makes one HTTP request to the CareerJet API.
func (s *Scraper) fetchPage(ctx context.Context, searchTerm, location, locale string, page, pageSize int) ([]byte, error) {
	u, err := url.Parse(s.apiURL)
	if err != nil {
		return nil, fmt.Errorf("careerjet url: %w", err)
	}

	q := u.Query()
	q.Set("affid", s.affID)
	q.Set("user_ip", "127.0.0.1")
	q.Set("user_agent", "scrappy/1.0 (+https://github.com/arinbalyan/scrappy)")
	q.Set("locale_code", locale)
	q.Set("keywords", searchTerm)
	q.Set("sort", "date")
	q.Set("page", fmt.Sprintf("%d", page))
	q.Set("pagesize", fmt.Sprintf("%d", pageSize))
	if strings.TrimSpace(location) != "" {
		q.Set("location", strings.TrimSpace(location))
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("careerjet request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("careerjet status %d", resp.StatusCode)
	}

	body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("careerjet read: %w", err)
	}
	return body, nil
}

// ─── Helpers ────────────────────────────────────────────────────────────────────

// resolveLocale maps a Country to a CareerJet locale code. Defaults to "en_US".
func resolveLocale(c model.Country) string {
	if loc, ok := countryToLocale[c]; ok {
		return loc
	}
	return "en_US"
}

// resolveSalaryInterval maps a CareerJet salary type code to a model interval.
// Defaults to yearly if the code is unknown.
func resolveSalaryInterval(v string) model.CompensationInterval {
	v = strings.TrimSpace(v)
	if v != "" {
		if interval, ok := salaryTypeMap[strings.ToUpper(v)]; ok {
			return interval
		}
	}
	return model.IntervalYearly
}

// parseLocation splits a "City, State" or "City, State, Country" location string.
func parseLocation(v string) model.Location {
	parts := strings.SplitN(v, ", ", 2)
	loc := model.Location{}
	if len(parts) > 0 {
		loc.City = strings.TrimSpace(parts[0])
	}
	if len(parts) > 1 {
		loc.State = strings.TrimSpace(parts[1])
	}
	return loc
}

// hashID generates a stable hash string for deduplication.

