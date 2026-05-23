package jobstreet

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/util"
)

const searchURL = "https://www.jobstreet.com/api/chalice-search/v4/search"

// salaryRE parses salary strings like "MYR 5,000 - MYR 8,000" or "MYR 5000-MYR 8000".
// Captures: currency1, minAmount, currency2, maxAmount.
var salaryRE = regexp.MustCompile(`([A-Z]{3})?\s*\$?([\d,]+)\s*[–-]\s*([A-Z]{3})?\s*\$?([\d,]+)`)

// Scraper scrapes JobStreet.com job listings via their Chalice search API.
type Scraper struct {
	client    *http.Client
	searchURL string
}

// New creates a new JobStreet scraper with the given HTTP client.
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
func (s *Scraper) SiteName() model.Site { return model.SiteJobStreet }

// Scrape fetches job listings from JobStreet with the given input.
func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	wanted := input.ResultsWanted
	if wanted <= 0 {
		wanted = 25
	}

	jobs := make([]model.JobPost, 0, wanted)

	body, err := s.fetchPage(ctx, input.SearchTerm, input.Location, wanted)
	if err != nil {
		return nil, fmt.Errorf("jobstreet: %w", err)
	}

	page, err := parseJobs(body)
	if err != nil {
		return nil, fmt.Errorf("jobstreet parse: %w", err)
	}

	for _, j := range page {
		jobs = append(jobs, j)
		if len(jobs) >= wanted {
			break
		}
	}

	if !util.HasMeaningfulJobs(jobs) {
		return nil, fmt.Errorf("jobstreet no parseable jobs")
	}
	if len(jobs) > wanted {
		jobs = jobs[:wanted]
	}
	return jobs, nil
}

// fetchPage calls the JobStreet Chalice search API.
func (s *Scraper) fetchPage(ctx context.Context, searchTerm, location string, pageSize int) ([]byte, error) {
	u, _ := url.Parse(s.searchURL)
	q := u.Query()
	q.Set("siteKey", "MY-Main")
	q.Set("pageSize", fmt.Sprintf("%d", pageSize))
	if v := strings.TrimSpace(searchTerm); v != "" {
		q.Set("keywords", v)
	}
	if v := strings.TrimSpace(location); v != "" {
		q.Set("where", v)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/129 Safari/537.36")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("jobstreet request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("jobstreet status %d", resp.StatusCode)
	}

	body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("jobstreet read: %w", err)
	}
	return body, nil
}

// parseJobs unmarshals the JSON response into JobPost records.
// The response can be either a raw array or an object with data/jobs fields.
func parseJobs(raw []byte) ([]model.JobPost, error) {
	rawJobs, err := unmarshalJobs(raw)
	if err != nil {
		return nil, err
	}
	if len(rawJobs) == 0 {
		return nil, nil
	}

	jobs := make([]model.JobPost, 0, len(rawJobs))
	for _, rj := range rawJobs {
		j, err := mapJob(rj)
		if err != nil {
			continue
		}
		if j != nil {
			jobs = append(jobs, *j)
		}
	}
	return jobs, nil
}

// unmarshalJobs handles both array and object response shapes.
func unmarshalJobs(raw []byte) ([]rawJob, error) {
	// Try direct array first.
	var arr []rawJob
	if err := json.Unmarshal(raw, &arr); err == nil {
		return arr, nil
	}

	// Try object with data or jobs field.
	var obj struct {
		Data []rawJob `json:"data"`
		Jobs []rawJob `json:"jobs"`
	}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	if len(obj.Data) > 0 {
		return obj.Data, nil
	}
	return obj.Jobs, nil
}

// mapJob converts a raw JobStreet job into a model.JobPost.
func mapJob(rj rawJob) (*model.JobPost, error) {
	if strings.TrimSpace(rj.Title) == "" {
		return nil, fmt.Errorf("missing title")
	}

	// Resolve company name: advertiser.description -> companyName -> company.
	companyName := rj.CompanyName
	if companyName == "" && rj.Advertiser != nil && strings.TrimSpace(rj.Advertiser.Description) != "" {
		companyName = rj.Advertiser.Description
	}
	if companyName == "" && rj.Company != "" {
		companyName = rj.Company
	}

	// Resolve job URL: listingUrl -> jobUrl -> construct from id.
	jobURL := strings.TrimSpace(rj.ListingURL)
	if jobURL == "" {
		jobURL = strings.TrimSpace(rj.JobURL)
	}
	if jobURL == "" && toStr(rj.ID) != "" {
		jobURL = fmt.Sprintf("https://www.jobstreet.com/job/%s", toStr(rj.ID))
	}
	if jobURL == "" {
		return nil, fmt.Errorf("missing job URL")
	}

	// ID: use provided id.
	idStr := toStr(rj.ID)
	if idStr == "" {
		return nil, fmt.Errorf("missing job ID")
	}
	fullID := fmt.Sprintf("jobstreet-%s", idStr)

	// Description: teaser -> description.
	description := ""
	if strings.TrimSpace(rj.Teaser) != "" {
		description = strings.TrimSpace(rj.Teaser)
	} else if strings.TrimSpace(rj.Description) != "" {
		description = strings.TrimSpace(rj.Description)
	}

	// Location: locationWhereValue -> location.
	locationStr := ""
	if strings.TrimSpace(rj.LocationWhereValue) != "" {
		locationStr = strings.TrimSpace(rj.LocationWhereValue)
	} else if strings.TrimSpace(rj.Location) != "" {
		locationStr = strings.TrimSpace(rj.Location)
	}

	// Salary.
	comp := parseSalary(rj.Salary, rj.SalaryLabel)

	// Date posted.
	var datePosted *time.Time
	if v := strings.TrimSpace(rj.ListingDate); v != "" {
		datePosted = util.ParseDatePosted(v)
	}

	// Classification (work type / department).
	department := ""
	if rj.Classification != nil && strings.TrimSpace(rj.Classification.Description) != "" {
		department = rj.Classification.Description
	}

	// Work type.
	jobType := ""
	if strings.TrimSpace(rj.WorkType) != "" {
		jobType = strings.TrimSpace(rj.WorkType)
	}

	return &model.JobPost{
		ID:          fullID,
		Title:       strings.TrimSpace(rj.Title),
		CompanyName: companyName,
		JobURL:      jobURL,
		Location:    model.Location{City: locationStr},
		Description: description,
		Compensation: comp,
		DatePosted:  datePosted,
		IsRemote:    rj.IsRemote,
		Department:  department,
		JobType:     jobType,
	}, nil
}

// parseSalary parses a salary string like "MYR 5,000 - MYR 8,000" into Compensation.
func parseSalary(salary, salaryLabel string) *model.Compensation {
	text := salary
	if strings.TrimSpace(text) == "" {
		text = salaryLabel
	}
	text = strings.TrimSpace(text)
	if text == "" || len(text) > 200 {
		return nil
	}

	m := salaryRE.FindStringSubmatch(text)
	if len(m) < 5 {
		return nil
	}

	currency := m[1]
	if currency == "" {
		currency = m[3]
	}
	if currency == "" {
		currency = "MYR"
	}

	minRaw := strings.ReplaceAll(m[2], ",", "")
	maxRaw := strings.ReplaceAll(m[4], ",", "")

	var minF, maxF float64
	if _, err := fmt.Sscanf(minRaw, "%f", &minF); err != nil || minF <= 0 {
		return nil
	}
	if _, err := fmt.Sscanf(maxRaw, "%f", &maxF); err != nil || maxF <= 0 {
		return nil
	}

	return &model.Compensation{
		Interval:  model.IntervalYearly,
		MinAmount: &minF,
		MaxAmount: &maxF,
		Currency:  currency,
	}
}

// --- JSON response types ---

// rawJob maps a single JobStreet job listing.
type rawJob struct {
	// ID can be a string or number in the API response.
	ID                 interface{}   `json:"id"`
	Title              string        `json:"title"`
	Advertiser         *advertiser   `json:"advertiser,omitempty"`
	CompanyName        string        `json:"companyName,omitempty"`
	Company            string        `json:"company,omitempty"`
	JobURL             string        `json:"jobUrl,omitempty"`
	ListingURL         string        `json:"listingUrl,omitempty"`
	Teaser             string        `json:"teaser,omitempty"`
	Description        string        `json:"description,omitempty"`
	Location           string        `json:"location,omitempty"`
	LocationWhereValue string        `json:"locationWhereValue,omitempty"`
	Salary             string        `json:"salary,omitempty"`
	SalaryLabel        string        `json:"salaryLabel,omitempty"`
	WorkType           string        `json:"workType,omitempty"`
	Classification     *classification `json:"classification,omitempty"`
	ListingDate        string        `json:"listingDate,omitempty"`
	IsRemote           bool          `json:"isRemote,omitempty"`
}

type advertiser struct {
	Description string `json:"description"`
}

type classification struct {
	Description string `json:"description"`
}

// toStr converts an interface{} value to a string for fields that may be
// either string or number in the API response (e.g. job IDs).
func toStr(v interface{}) string {
	if v == nil {
		return ""
	}
	switch s := v.(type) {
	case string:
		return s
	case float64:
		return fmt.Sprintf("%.0f", s)
	case json.Number:
		return s.String()
	default:
		return fmt.Sprintf("%v", s)
	}
}
