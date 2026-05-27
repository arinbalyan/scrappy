package ziprecruiter

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
	baseURL          = "https://api.ziprecruiter.com/jobs-app/jobs"
	eventURL         = "https://api.ziprecruiter.com/jobs-app/event"
	defaultResults   = 15
	defaultPageSize  = 20
	minDelay         = 5 * time.Second
	bandDelay        = 5 * time.Second
)

var defaultHeaders = map[string]string{
	"Host":                       "api.ziprecruiter.com",
	"Accept-Encoding":            "gzip",
	"Authorization":              "Basic YTBlMTk4NjItMjhiYi00YmU3LTlhZDAtZGNhZGMwZjBmY2M5Og==",
	"X-Zr-Zva-Override":          "utm_source:CARNIVAL;utm_medium:NLX",
	"User-Agent":                 "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36",
}

// --- API response types ---

// apiResponse maps the ZipRecruiter API response.
type apiResponse struct {
	Jobs          []jobEntry `json:"jobs"`
	ContinueToken *string    `json:"continue_token"`
}

// jobEntry maps a single ZipRecruiter job.
type jobEntry struct {
	JobID       string       `json:"job_id"`
	Name        string       `json:"name"`
	Title       string       `json:"title"`
	Snippet     string       `json:"snippet"`
	JobDesc     string       `json:"job_description"`
	JobCity     string       `json:"job_city"`
	JobState    string       `json:"job_state"`
	JobCountry  string       `json:"job_country"`
	JobURL      string       `json:"job_url"`
	URL         string       `json:"url"`
	ApplyURL    string       `json:"apply_url"`
	PostedTime  string       `json:"posted_time"`
	Remote      string       `json:"remote"`
	EmploymentType string    `json:"employment_type"`
	SalaryMin   *float64     `json:"salary_min_annual"`
	SalaryMax   *float64     `json:"salary_max_annual"`
	HiringCompany *hiringCompany `json:"hiring_company"`
}

type hiringCompany struct {
	Name string `json:"name"`
	URL  string `json:"url"`
	Logo string `json:"logo"`
}

// Scraper fetches jobs from the ZipRecruiter API.
type Scraper struct {
	client *http.Client
}

// New creates a new ZipRecruiter scraper.
func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Timeout: 20 * time.Second})
	}
	return &Scraper{client: client}
}

// SiteName returns the site identifier.
func (s *Scraper) SiteName() model.Site { return model.SiteZipRecruiter }

// Scrape fetches jobs from the ZipRecruiter API with cursor-based pagination.
func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{
		"site":           s.SiteName(),
		"results_wanted": input.ResultsWanted,
		"search_term":    input.SearchTerm,
	})

	wanted := input.ResultsWanted
	if wanted <= 0 {
		wanted = defaultResults
	}

	// Initialize session (best-effort, non-fatal)
	initSession(ctx, s.client)

	jobs := make([]model.JobPost, 0, wanted)
	seenIDs := make(map[string]bool)
	var continueToken *string

	for len(jobs) < wanted {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		pageURL := buildPageURL(input, continueToken)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
		if err != nil {
			return nil, fmt.Errorf("ziprecruiter: build request: %w", err)
		}
		for k, v := range defaultHeaders {
			req.Header.Set(k, v)
		}

		resp, err := s.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("ziprecruiter: request: %w", err)
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			resp.Body.Close()
			return nil, fmt.Errorf("ziprecruiter: status %d — try using --proxy with a residential proxy", resp.StatusCode)
		}

		body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("ziprecruiter: read: %w", err)
		}

		var parsed apiResponse
		if err := json.Unmarshal(body, &parsed); err != nil {
			return nil, fmt.Errorf("ziprecruiter: decode: %w", err)
		}

		if len(parsed.Jobs) == 0 {
			break
		}

		for _, raw := range parsed.Jobs {
			if len(jobs) >= wanted {
				break
			}
			jobID := raw.JobID
			if jobID == "" {
				continue
			}
			if seenIDs[jobID] {
				continue
			}
			seenIDs[jobID] = true

			job := mapJob(raw)
			if strings.TrimSpace(job.Title) == "" {
				continue
			}
			jobs = append(jobs, job)
		}

		continueToken = parsed.ContinueToken
		if continueToken == nil || *continueToken == "" {
			break
		}

		util.JitterSleep(ctx, minDelay, bandDelay)
	}

	util.Debug("scraper_done", map[string]any{
		"site": s.SiteName(),
		"jobs": len(jobs),
	})

	if !util.HasMeaningfulJobs(jobs) {
		return nil, fmt.Errorf("ziprecruiter: no parseable jobs")
	}
	return jobs, nil
}

// initSession sends the session initialization event (best-effort).
func initSession(ctx context.Context, client *http.Client) {
	data := map[string]string{
		"device_make":        "Apple",
		"device_model":       "Macintosh",
		"device_os":          "macOS",
		"event_type":         "session",
		"device_form_factor": "desktop",
		"platform":           "web",
	}
	enc, _ := json.Marshal(data)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, eventURL, strings.NewReader(string(enc)))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range defaultHeaders {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err == nil {
		resp.Body.Close()
	}
}

// buildPageURL builds the ZipRecruiter API URL with query parameters.
func buildPageURL(input model.ScraperInput, continueToken *string) string {
	q := url.Values{}
	q.Set("search", input.SearchTerm)
	q.Set("location", input.Location)
	q.Set("radius_miles", fmt.Sprintf("%d", input.DistanceMiles))
	q.Set("form", "jobs-landing")

	if continueToken != nil && *continueToken != "" {
		q.Set("continue_token", *continueToken)
	}
	if input.HoursOld > 0 {
		q.Set("days_ago", fmt.Sprintf("%d", (input.HoursOld+23)/24))
	}
	if input.JobType != "" {
		if t := toZRJobType(string(input.JobType)); t != "" {
			q.Set("employment_type", t)
		}
	}
	return baseURL + "?" + q.Encode()
}

// toZRJobType converts scrappy job type to ZipRecruiter employment_type value.
func toZRJobType(jt string) string {
	switch strings.ToLower(strings.TrimSpace(jt)) {
	case "fulltime":
		return "full_time"
	case "parttime":
		return "part_time"
	case "contract":
		return "contractor"
	case "internship":
		return "intern"
	case "temporary":
		return "temporary"
	default:
		return ""
	}
}

// mapJob converts a raw ZipRecruiter API job to a model.JobPost.
func mapJob(raw jobEntry) model.JobPost {
	title := raw.Name
	if title == "" {
		title = raw.Title
	}

	jobURL := raw.JobURL
	if jobURL == "" {
		jobURL = raw.URL
	}

	// Description
	desc := raw.JobDesc
	if desc == "" {
		desc = raw.Snippet
	}

	// Location
	location := model.Location{
		City:    strings.TrimSpace(raw.JobCity),
		State:   strings.TrimSpace(raw.JobState),
		Country: strings.TrimSpace(raw.JobCountry),
	}

	// Compensation
	var comp *model.Compensation
	if raw.SalaryMin != nil || raw.SalaryMax != nil {
		comp = &model.Compensation{
			Interval:  model.IntervalYearly,
			MinAmount: raw.SalaryMin,
			MaxAmount: raw.SalaryMax,
			Currency:  "USD",
		}
	}

	// DatePosted
	var datePosted *time.Time
	if raw.PostedTime != "" {
		datePosted = parseDate(raw.PostedTime)
	}

	// Company
	companyName := ""
	companyURL := ""
	companyLogo := ""
	if raw.HiringCompany != nil {
		companyName = strings.TrimSpace(raw.HiringCompany.Name)
		companyURL = strings.TrimSpace(raw.HiringCompany.URL)
		companyLogo = strings.TrimSpace(raw.HiringCompany.Logo)
	}

	// IsRemote
	isRemote := strings.ToLower(raw.Remote) == "true"

	// JobType
	jobType := ""
	if raw.EmploymentType != "" {
		jobType = normalizeJobType(raw.EmploymentType)
	}

	job := model.JobPost{
		ID:            "zr-" + raw.JobID,
		Title:         strings.TrimSpace(title),
		CompanyName:   companyName,
		CompanyURL:    companyURL,
		JobURL:        jobURL,
		JobURLDirect:  strings.TrimSpace(raw.ApplyURL),
		Location:      location,
		IsRemote:      isRemote,
		Description:   desc,
		JobType:       jobType,
		DatePosted:    datePosted,
		Compensation:  comp,
		CompanyLogo:   companyLogo,
		Site:          string(model.SiteZipRecruiter),
	}
	return job
}

// normalizeJobType maps ZipRecruiter employment_type to scrappy job type.
func normalizeJobType(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "full_time", "fulltime":
		return "fulltime"
	case "part_time", "parttime":
		return "parttime"
	case "contractor", "contract":
		return "contract"
	case "intern", "internship":
		return "internship"
	case "temporary", "temp":
		return "temporary"
	default:
		return ""
	}
}

// parseDate attempts to parse a date string in various formats.
func parseDate(v string) *time.Time {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05",
		"2006-01-02",
		time.RFC1123Z,
		time.RFC1123,
	}
	for _, f := range formats {
		t, err := time.Parse(f, v)
		if err == nil {
			return &t
		}
	}
	return nil
}


