package usajobs

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/util"
)

const (
	apiURL        = "https://data.usajobs.gov/api/Search"
	maxPageSize   = 500
	defaultWanted = 25
	maxPages      = 5
)

var stripHTMLRe = regexp.MustCompile(`(?is)<[^>]+>`)

// ---- API response types ----

type searchResponse struct {
	SearchResult *searchResult `json:"SearchResult,omitempty"`
}

type searchResult struct {
	SearchResultCount    int            `json:"SearchResultCount"`
	SearchResultCountAll int            `json:"SearchResultCountAll"`
	SearchResultItems    []searchItem   `json:"SearchResultItems,omitempty"`
}

type searchItem struct {
	MatchedObjectID       string       `json:"MatchedObjectId"`
	MatchedObjectDescriptor *jobDescriptor `json:"MatchedObjectDescriptor,omitempty"`
}

type jobDescriptor struct {
	PositionTitle         string          `json:"PositionTitle"`
	PositionURI           string          `json:"PositionURI"`
	PositionID            string          `json:"PositionID"`
	OrganizationName      string          `json:"OrganizationName"`
	DepartmentName        string          `json:"DepartmentName"`
	PositionLocation      []apiLocation   `json:"PositionLocation,omitempty"`
	PositionRemuneration   []apiRemuneration `json:"PositionRemuneration,omitempty"`
	PublicationStartDate  string          `json:"PublicationStartDate"`
	ApplicationCloseDate  string          `json:"ApplicationCloseDate"`
	QualificationSummary  string          `json:"QualificationSummary"`
	UserArea              *userArea       `json:"UserArea,omitempty"`
}

type apiLocation struct {
	LocationName           string `json:"LocationName"`
	CountryCode            string `json:"CountryCode"`
	CityName               string `json:"CityName"`
	CountrySubDivisionCode string `json:"CountrySubDivisionCode"`
}

type apiRemuneration struct {
	MinimumRange     string `json:"MinimumRange"`
	MaximumRange     string `json:"MaximumRange"`
	RateIntervalCode string `json:"RateIntervalCode"`
	Description      string `json:"Description"`
}

type userArea struct {
	Details *userAreaDetails `json:"Details,omitempty"`
}

type userAreaDetails struct {
	JobSummary  string   `json:"JobSummary"`
	MajorDuties []string `json:"MajorDuties,omitempty"`
}

// Scraper fetches jobs from the USAJobs API.
type Scraper struct {
	client  *http.Client
	apiURL  string
	apiKey  string
	email   string
}

// New creates a new USAJobs scraper.
// Uses USAJOBS_API_KEY and USAJOBS_EMAIL env vars for authentication.
func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Timeout: 20 * time.Second})
	}
	return &Scraper{
		client: client,
		apiURL: apiURL,
		apiKey: os.Getenv("USAJOBS_API_KEY"),
		email:  os.Getenv("USAJOBS_EMAIL"),
	}
}

// NewWithCredentials creates a new scraper with explicit credentials (used in tests).
func NewWithCredentials(client *http.Client, apiURL, apiKey, email string) *Scraper {
	s := New(client)
	s.apiKey = apiKey
	s.email = email
	if strings.TrimSpace(apiURL) != "" {
		s.apiURL = apiURL
	}
	return s
}

// SiteName returns the site identifier.
func (s *Scraper) SiteName() model.Site { return model.SiteUSAJobs }

// IsConfigured reports whether the required credentials are set.
func (s *Scraper) IsConfigured() bool {
	return s.apiKey != "" && s.email != ""
}

// Scrape fetches jobs from the USAJobs API.
func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{
		"site":           s.SiteName(),
		"results_wanted": input.ResultsWanted,
		"search_term":    input.SearchTerm,
		"location":       input.Location,
	})

	if !s.IsConfigured() {
		return nil, fmt.Errorf("usajobs: not configured; set USAJOBS_API_KEY and USAJOBS_EMAIL env vars")
	}

	wanted := input.ResultsWanted
	if wanted <= 0 {
		wanted = defaultWanted
	}

	pageSize := wanted
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}

	term := strings.TrimSpace(input.SearchTerm)
	location := strings.TrimSpace(input.Location)

	jobs := make([]model.JobPost, 0, wanted)
	seen := make(map[string]bool)
	page := 1

	for page <= maxPages && len(jobs) < wanted {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		items, err := s.fetchPage(ctx, term, location, pageSize, page)
		if err != nil {
			return nil, fmt.Errorf("usajobs: page %d: %w", page, err)
		}
		if len(items) == 0 {
			break
		}

		for _, item := range items {
			if len(jobs) >= wanted {
				break
			}
			if seen[item.MatchedObjectID] {
				continue
			}
			seen[item.MatchedObjectID] = true

			job, err := mapJob(item)
			if err != nil {
				continue
			}
			jobs = append(jobs, job)
		}

		// Stop if fewer results than page size (last page)
		if len(items) < pageSize {
			break
		}
		page++
	}

	util.Debug("scraper_done", map[string]any{
		"site": s.SiteName(),
		"jobs": len(jobs),
	})

	if !util.HasMeaningfulJobs(jobs) {
		return nil, fmt.Errorf("usajobs: no parseable jobs")
	}
	return jobs, nil
}

// fetchPage fetches one page of results from the USAJobs API.
func (s *Scraper) fetchPage(ctx context.Context, keyword, location string, pageSize, page int) ([]searchItem, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.apiURL, nil)
	if err != nil {
		return nil, err
	}

	q := req.URL.Query()
	q.Set("Keyword", keyword)
	q.Set("ResultsPerPage", strconv.Itoa(pageSize))
	q.Set("Page", strconv.Itoa(page))
	q.Set("Fields", "Full")
	if location != "" {
		q.Set("LocationName", location)
	}
	req.URL.RawQuery = q.Encode()

	req.Header.Set("Host", "data.usajobs.gov")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization-Key", s.apiKey)
	req.Header.Set("User-Agent", s.email)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("read: %w", err)
	}

	var parsed searchResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	if parsed.SearchResult == nil {
		return nil, nil
	}

	return parsed.SearchResult.SearchResultItems, nil
}

// mapJob converts a USAJobs search item to a JobPost.
func mapJob(item searchItem) (model.JobPost, error) {
	desc := item.MatchedObjectDescriptor
	if desc == nil {
		return model.JobPost{}, fmt.Errorf("no descriptor")
	}

	title := strings.TrimSpace(desc.PositionTitle)
	jobURL := strings.TrimSpace(desc.PositionURI)
	if title == "" || jobURL == "" {
		return model.JobPost{}, fmt.Errorf("empty title or URL")
	}

	// Build description
	description := ""
	if desc.UserArea != nil && desc.UserArea.Details != nil {
		details := desc.UserArea.Details
		if strings.TrimSpace(details.JobSummary) != "" {
			description = strings.TrimSpace(details.JobSummary)
		}
		if len(details.MajorDuties) > 0 {
			duties := make([]string, 0, len(details.MajorDuties))
			for _, d := range details.MajorDuties {
				if strings.TrimSpace(d) != "" {
					duties = append(duties, "- "+strings.TrimSpace(stripHTML(d)))
				}
			}
			if len(duties) > 0 {
				dutiesText := strings.Join(duties, "\n")
				if description != "" {
					description = description + "\n\nMajor Duties:\n" + dutiesText
				} else {
					description = "Major Duties:\n" + dutiesText
				}
			}
		}
	}
	if description == "" && strings.TrimSpace(desc.QualificationSummary) != "" {
		description = strings.TrimSpace(desc.QualificationSummary)
	}

	// Build location
	location := model.Location{}
	if len(desc.PositionLocation) > 0 {
		loc := desc.PositionLocation[0]
		location.City = strings.TrimSpace(loc.CityName)
		location.State = strings.TrimSpace(loc.CountrySubDivisionCode)
		location.Country = strings.TrimSpace(loc.CountryCode)
	}

	// Build compensation
	var comp *model.Compensation
	if len(desc.PositionRemuneration) > 0 {
		comp = mapCompensation(desc.PositionRemuneration[0])
	}

	// Parse date
	var datePosted *time.Time
	if strings.TrimSpace(desc.PublicationStartDate) != "" {
		datePosted = parseDate(desc.PublicationStartDate)
	}

	return model.JobPost{
		ID:           "usajobs-" + item.MatchedObjectID,
		Title:        title,
		CompanyName:  strings.TrimSpace(desc.OrganizationName),
		JobURL:       jobURL,
		Location:     location,
		Description:  description,
		Compensation: comp,
		DatePosted:   datePosted,
		Site:         string(model.SiteUSAJobs),
		ApplyMethod:  "external_url",
	}, nil
}

// mapCompensation converts USAJobs remuneration data.
func mapCompensation(r apiRemuneration) *model.Compensation {
	minVal, errMin := strconv.ParseFloat(strings.TrimSpace(r.MinimumRange), 64)
	maxVal, errMax := strconv.ParseFloat(strings.TrimSpace(r.MaximumRange), 64)

	if errMin != nil && errMax != nil {
		return nil
	}

	interval := mapInterval(r.Description)
	if interval == "" {
		interval = mapInterval(r.RateIntervalCode)
	}
	if interval == "" {
		interval = "yearly"
	}

	comp := &model.Compensation{
		Interval: model.CompensationInterval(interval),
		Currency: "USD",
	}
	if errMin == nil {
		comp.MinAmount = &minVal
	}
	if errMax == nil {
		comp.MaxAmount = &maxVal
	}
	return comp
}

// mapInterval maps USAJobs rate interval descriptions to model intervals.
func mapInterval(desc string) string {
	switch strings.TrimSpace(desc) {
	case "Per Year", "PA":
		return "yearly"
	case "Per Month", "PM":
		return "monthly"
	case "Per Week", "PW":
		return "weekly"
	case "Per Day", "PD":
		return "daily"
	case "Per Hour", "PH":
		return "hourly"
	default:
		return ""
	}
}

// stripHTML removes HTML tags.
func stripHTML(s string) string {
	return strings.TrimSpace(stripHTMLRe.ReplaceAllString(s, " "))
}

// parseDate parses a date string.
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
	}
	for _, f := range formats {
		t, err := time.Parse(f, v)
		if err == nil {
			return &t
		}
	}
	return nil
}
