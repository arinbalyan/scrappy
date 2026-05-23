package fwdayweek

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

const defaultAPI = "https://4dayweek.io/api/jobs"
const maxPages = 5

// ---------- API response types ----------

type apiResponse struct {
	Jobs    []apiJob `json:"jobs"`
	Total   int      `json:"total"`
	Page    int      `json:"page"`
	HasMore bool     `json:"has_more"`
}

type apiJob struct {
	ID             string       `json:"id"`
	Title          string       `json:"title"`
	Slug           string       `json:"slug"`
	CompanyName    string       `json:"company_name"`
	WorkArrangement string      `json:"work_arrangement"` // remote | hybrid | onsite
	Locations      []apiLocation `json:"locations,omitempty"`
	Posted         int64        `json:"posted"` // unix timestamp
	ScheduleType   string       `json:"schedule_type"`
	Salary         string       `json:"salary,omitempty"`
	SalaryLower    int64        `json:"salary_lower,omitempty"`
	SalaryUpper    int64        `json:"salary_upper,omitempty"`
	SalaryCurrency string       `json:"salary_currency,omitempty"`
	SalaryPeriod   string       `json:"salary_period,omitempty"`
	Stack          []apiStack   `json:"stack,omitempty"`
	Category       string       `json:"category,omitempty"`
	Level          string       `json:"level,omitempty"`
	Company        apiCompany   `json:"company"`
	WorkLifeScore  int          `json:"work_life_score"`
}

type apiLocation struct {
	City    string `json:"city,omitempty"`
	Country string `json:"country,omitempty"`
}

type apiStack struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type apiCompany struct {
	Name    string `json:"name"`
	Slug    string `json:"slug"`
	LogoURL string `json:"logo_url,omitempty"`
}

// ---------- Scraper ----------

type Scraper struct {
	client *http.Client
	apiURL string
}

func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Retries: 2, Timeout: 20 * time.Second})
	}
	return &Scraper{client: client, apiURL: defaultAPI}
}

func NewWithAPIURL(client *http.Client, endpoint string) *Scraper {
	s := New(client)
	if strings.TrimSpace(endpoint) != "" {
		s.apiURL = strings.TrimSpace(endpoint)
	}
	return s
}

func (s *Scraper) SiteName() model.Site { return model.Site4DayWeek }

func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	wanted := input.ResultsWanted
	if wanted <= 0 {
		wanted = 25
	}

	jobs := make([]model.JobPost, 0, wanted)
	jobsSeen := make(map[string]bool)
	nextURL := s.apiURL
	page := 0

	for len(jobs) < wanted {
		select {
		case <-ctx.Done():
			return jobs, ctx.Err()
		default:
		}

		page++
		if page > maxPages {
			break
		}

		pageJobs, hasMore, err := s.fetchPage(ctx, nextURL)
		if err != nil {
			return nil, fmt.Errorf("4dayweek page %d: %w", page, err)
		}

		if len(pageJobs) > 0 {
			for _, j := range pageJobs {
				if len(jobs) >= wanted {
					break
				}
				if jobsSeen[j.ID] {
					continue
				}
				jobsSeen[j.ID] = true
				job := s.toJobPost(j)
				if strings.TrimSpace(job.Title) == "" {
					continue
				}
				jobs = append(jobs, job)
			}
		}

		if len(pageJobs) == 0 || !hasMore {
			break
		}
		nextURL = s.buildNextPageURL(s.apiURL, page)
	}

	if !util.HasMeaningfulJobs(jobs) {
		return nil, fmt.Errorf("4dayweek no parseable jobs")
	}
	return jobs, nil
}

func (s *Scraper) fetchPage(ctx context.Context, apiURL string) ([]apiJob, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, false, fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, false, fmt.Errorf("status %d", resp.StatusCode)
	}

	body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return nil, false, fmt.Errorf("read: %w", err)
	}

	var parsed apiResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, false, fmt.Errorf("decode: %w", err)
	}
	return parsed.Jobs, parsed.HasMore, nil
}

// buildNextPageURL constructs the next page URL from the API base and current page number.
func (s *Scraper) buildNextPageURL(base string, page int) string {
	u, _ := url.Parse(base)
	q := u.Query()
	q.Set("page", strconv.Itoa(page+1))
	u.RawQuery = q.Encode()
	return u.String()
}

func (s *Scraper) toJobPost(j apiJob) model.JobPost {
	jobURL := fmt.Sprintf("https://4dayweek.io/jobs/%s", j.Slug)

	loc := s.parseLocation(j.Locations)
	isRemote := strings.EqualFold(j.WorkArrangement, "remote") || j.WorkArrangement == ""

	var posted *time.Time
	if j.Posted > 0 {
		t := time.Unix(j.Posted, 0).UTC()
		posted = &t
	}

	job := model.JobPost{
		ID:          "4dw-" + j.ID,
		Title:       strings.TrimSpace(j.Title),
		CompanyName: strings.TrimSpace(j.CompanyName),
		CompanyURL:  "https://4dayweek.io/companies/" + j.Company.Slug,
		JobURL:      jobURL,
		Location:    loc,
		IsRemote:    isRemote,
		DatePosted:  posted,
		Department:  strings.TrimSpace(j.Category),
		Seniority:   strings.TrimSpace(j.Level),
	}

	if j.Company.LogoURL != "" {
		job.CompanyLogoURL = j.Company.LogoURL
	}

	// Map schedule_type -> ApplyMethod
	switch j.ScheduleType {
	case "4_day_week":
		job.ApplyMethod = "4-day week (100% pay)"
	case "4_day_week_pro_rata":
		job.ApplyMethod = "4-day week (pro-rata)"
	case "9_day_fortnight":
		job.ApplyMethod = "9-day fortnight"
	case "flex_fridays":
		job.ApplyMethod = "Flex Fridays"
	case "flexible_hours":
		job.ApplyMethod = "Flexible hours"
	case "summer_fridays":
		job.ApplyMethod = "Summer Fridays"
	case "rotating_4_day":
		job.ApplyMethod = "Rotating 4-day week"
	case "generous_pto":
		job.ApplyMethod = "Generous PTO"
	case "compressed_week":
		job.ApplyMethod = "Compressed week"
	case "unlimited_pto":
		job.ApplyMethod = "Unlimited PTO"
	case "half_day_fridays":
		job.ApplyMethod = "Half-day Fridays"
	}

	// Parse salary
	if j.SalaryLower > 0 || j.SalaryUpper > 0 || j.Salary != "" {
		comp := &model.Compensation{
			Currency: "USD",
		}
		if j.SalaryCurrency != "" {
			comp.Currency = j.SalaryCurrency
		}
		switch j.SalaryPeriod {
		case "year":
			comp.Interval = model.IntervalYearly
		case "month":
			comp.Interval = model.IntervalMonthly
		case "hour":
			comp.Interval = model.IntervalHourly
		default:
			comp.Interval = model.IntervalYearly
		}
		if j.SalaryLower > 0 {
			v := float64(j.SalaryLower) / 100
			comp.MinAmount = &v
		}
		if j.SalaryUpper > 0 {
			v := float64(j.SalaryUpper) / 100
			comp.MaxAmount = &v
		}
		job.Compensation = comp
	}

	return job
}

func (s *Scraper) parseLocation(locs []apiLocation) model.Location {
	if len(locs) == 0 {
		return model.Location{}
	}
	// Use the first primary location or just the first one
	loc := locs[0]
	return model.Location{
		City:    strings.TrimSpace(loc.City),
		Country: strings.TrimSpace(loc.Country),
	}
}
