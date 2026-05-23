package ziprecruiter

import (
	"bytes"
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

const (
	defaultListURL  = "https://api.ziprecruiter.com/jobs-app/jobs"
	defaultEventURL = "https://api.ziprecruiter.com/jobs-app/event"
)

type Scraper struct {
	client   *http.Client
	listURL  string
	eventURL string
}

func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Retries: 3, CookieResetEveryN: 120, Timeout: 18 * time.Second})
	}
	return &Scraper{client: client, listURL: defaultListURL, eventURL: defaultEventURL}
}

func NewWithURLs(client *http.Client, listURL, eventURL string) *Scraper {
	s := New(client)
	if strings.TrimSpace(listURL) != "" {
		s.listURL = listURL
	}
	if strings.TrimSpace(eventURL) != "" {
		s.eventURL = eventURL
	}
	return s
}

func (s *Scraper) SiteName() model.Site { return model.SiteZipRecruiter }

func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	if input.ResultsWanted <= 0 {
		input.ResultsWanted = 15
	}
	_ = s.initSession(ctx)

	seen := make(map[string]struct{})
	jobs := make([]model.JobPost, 0, input.ResultsWanted)
	var continueToken string

	for len(jobs) < input.ResultsWanted {
		page, nextToken, err := s.fetchPage(ctx, input, continueToken, seen)
		if err != nil {
			return nil, err
		}
		if len(page) == 0 {
			break
		}
		jobs = append(jobs, page...)
		if nextToken == "" || nextToken == continueToken {
			break
		}
		continueToken = nextToken
	}

	if !util.HasMeaningfulJobs(jobs) {
		return nil, fmt.Errorf("zip_recruiter no parseable jobs")
	}
	if len(jobs) > input.ResultsWanted {
		jobs = jobs[:input.ResultsWanted]
	}
	return jobs, nil
}

func (s *Scraper) initSession(ctx context.Context) error {
	payload := map[string]string{
		"device_make":        "Apple",
		"device_model":       "Macintosh",
		"device_os":          "macOS",
		"event_type":         "session",
		"device_form_factor": "desktop",
		"platform":           "web",
	}
	b, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.eventURL, bytes.NewReader(b))
	if err != nil {
		return err
	}
	applyHeaders(req)
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func (s *Scraper) fetchPage(ctx context.Context, input model.ScraperInput, continueToken string, seen map[string]struct{}) ([]model.JobPost, string, error) {
	u, _ := url.Parse(s.listURL)
	q := u.Query()
	q.Set("search", strings.TrimSpace(input.SearchTerm))
	q.Set("location", strings.TrimSpace(input.Location))
	r := input.DistanceMiles
	if r <= 0 {
		r = 50
	}
	q.Set("radius_miles", strconv.Itoa(r))
	q.Set("form", "jobs-landing")
	if continueToken != "" {
		q.Set("continue_token", continueToken)
	}
	if input.HoursOld > 0 {
		days := (input.HoursOld + 23) / 24
		q.Set("days_ago", strconv.Itoa(days))
	}
	if jt := employmentType(input.JobType); jt != "" {
		q.Set("employment_type", jt)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, "", fmt.Errorf("zip_recruiter request: %w", err)
	}
	applyHeaders(req)
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("zip_recruiter request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("zip_recruiter status %d", resp.StatusCode)
	}

	var parsed struct {
		Jobs []struct {
			ID              interface{} `json:"id"`
			JobID           interface{} `json:"job_id"`
			Name            string      `json:"name"`
			Title           string      `json:"title"`
			JobURL          string      `json:"job_url"`
			URL             string      `json:"url"`
			ApplyURL        string      `json:"apply_url"`
			JobDescription  string      `json:"job_description"`
			Snippet         string      `json:"snippet"`
			PostedTime      string      `json:"posted_time"`
			EmploymentType  string      `json:"employment_type"`
			Remote          string      `json:"remote"`
			JobCity         string      `json:"job_city"`
			JobState        string      `json:"job_state"`
			JobCountry      string      `json:"job_country"`
			SalaryMinAnnual *float64    `json:"salary_min_annual"`
			SalaryMaxAnnual *float64    `json:"salary_max_annual"`
			HiringCompany   struct {
				Name string `json:"name"`
				URL  string `json:"url"`
				Logo string `json:"logo"`
			} `json:"hiring_company"`
		} `json:"jobs"`
		ContinueToken string `json:"continue_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, "", fmt.Errorf("zip_recruiter decode: %w", err)
	}

	out := make([]model.JobPost, 0, len(parsed.Jobs))
	for i, j := range parsed.Jobs {
		title := strings.TrimSpace(j.Name)
		if title == "" {
			title = strings.TrimSpace(j.Title)
		}
		if title == "" {
			continue
		}
		jobURL := strings.TrimSpace(j.JobURL)
		if jobURL == "" {
			jobURL = strings.TrimSpace(j.URL)
		}
		if jobURL == "" {
			continue
		}
		id := extractID(j.JobID)
		if id == "" {
			id = extractID(j.ID)
		}
		if id == "" {
			id = fmt.Sprintf("%d", i+1)
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}

		post := model.JobPost{
			ID:           "zr-" + id,
			Title:        title,
			CompanyName:  strings.TrimSpace(j.HiringCompany.Name),
			CompanyURL:   strings.TrimSpace(j.HiringCompany.URL),
			CompanyLogo:  strings.TrimSpace(j.HiringCompany.Logo),
			JobURL:       jobURL,
			JobURLDirect: strings.TrimSpace(j.ApplyURL),
			Location: model.Location{
				City:    strings.TrimSpace(j.JobCity),
				State:   strings.TrimSpace(j.JobState),
				Country: strings.TrimSpace(j.JobCountry),
			},
			Description: strings.TrimSpace(firstNonEmpty(j.JobDescription, j.Snippet)),
			IsRemote:    strings.EqualFold(strings.TrimSpace(j.Remote), "true"),
			JobType:     normalizeJobType(j.EmploymentType),
		}
		if post.CompanyName == "" {
			post.CompanyName = "ZipRecruiter"
		}
		if t := parsePostedTime(j.PostedTime); t != nil {
			post.DatePosted = t
		}
		if j.SalaryMinAnnual != nil || j.SalaryMaxAnnual != nil {
			post.Compensation = &model.Compensation{Interval: model.IntervalYearly, MinAmount: j.SalaryMinAnnual, MaxAmount: j.SalaryMaxAnnual, Currency: "USD"}
		}
		out = append(out, post)
	}

	return out, strings.TrimSpace(parsed.ContinueToken), nil
}

func applyHeaders(req *http.Request) {
	req.Header.Set("Host", "api.ziprecruiter.com")
	req.Header.Set("Accept-Encoding", "gzip")
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Authorization", "Basic Og==")
	req.Header.Set("x-zr-zva-override", "utm_source:CARNIVAL;utm_medium:NLX")
}

func employmentType(t model.JobType) string {
	switch t {
	case model.JobTypeFullTime:
		return "full_time"
	case model.JobTypePartTime:
		return "part_time"
	case model.JobTypeContract:
		return "contractor"
	case model.JobTypeInternship:
		return "intern"
	case model.JobTypeTemporary:
		return "temporary"
	default:
		return ""
	}
}

func normalizeJobType(v string) string {
	s := strings.ToLower(strings.TrimSpace(v))
	s = strings.ReplaceAll(s, "_", "")
	switch s {
	case "fulltime":
		return string(model.JobTypeFullTime)
	case "parttime":
		return string(model.JobTypePartTime)
	case "contractor", "contract":
		return string(model.JobTypeContract)
	case "intern", "internship":
		return string(model.JobTypeInternship)
	case "temporary":
		return string(model.JobTypeTemporary)
	default:
		return s
	}
}

func firstNonEmpty(v ...string) string {
	for _, s := range v {
		if strings.TrimSpace(s) != "" {
			return s
		}
	}
	return ""
}

func extractID(v interface{}) string {
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case float64:
		return strconv.FormatInt(int64(t), 10)
	case int:
		return strconv.Itoa(t)
	default:
		return ""
	}
}

func parsePostedTime(v string) *time.Time {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	if t, err := time.Parse(time.RFC3339, v); err == nil {
		return &t
	}
	return nil
}
