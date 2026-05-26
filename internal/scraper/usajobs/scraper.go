package usajobs

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/util"
)

const (
	apiURL          = "https://data.usajobs.gov/api/Search"
	maxPageSize     = 500
	defaultWanted   = 25
)

// Scraper fetches jobs from the USAJobs API.
type Scraper struct {
	client *http.Client
	apiURL string
}

// New creates a new USAJobs scraper.
func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{
			Timeout: 30 * time.Second,
			Retries: 2,
		})
	}
	return &Scraper{client: client, apiURL: apiURL}
}

// NewWithAPIURL creates a scraper with a custom endpoint (used in tests).
func NewWithAPIURL(client *http.Client, endpoint string) *Scraper {
	s := New(client)
	if strings.TrimSpace(endpoint) != "" {
		s.apiURL = strings.TrimSpace(endpoint)
	}
	return s
}

// SiteName returns the site identifier.
func (s *Scraper) SiteName() model.Site { return model.SiteUSAJobs }

// --- API response types ---

type usaJobsResponse struct {
	SearchResult *searchResult `json:"SearchResult,omitempty"`
}

type searchResult struct {
	SearchResultCount    int        `json:"SearchResultCount"`
	SearchResultCountAll int        `json:"SearchResultCountAll"`
	SearchResultItems    []usaJobsItem `json:"SearchResultItems"`
}

type usaJobsItem struct {
	MatchedObjectId       string            `json:"MatchedObjectId"`
	MatchedObjectDescriptor *jobDescriptor `json:"MatchedObjectDescriptor,omitempty"`
}

type jobDescriptor struct {
	PositionTitle       string           `json:"PositionTitle"`
	PositionURI         string           `json:"PositionURI"`
	OrganizationName    string           `json:"OrganizationName"`
	PositionLocation    []usaJobsLocation `json:"PositionLocation,omitempty"`
	PositionRemuneration []usaJobsRemuneration `json:"PositionRemuneration,omitempty"`
	PublicationStartDate string          `json:"PublicationStartDate"`
	QualificationSummary string          `json:"QualificationSummary"`
	UserArea            *userArea       `json:"UserArea,omitempty"`
}

type usaJobsLocation struct {
	CityName             string `json:"CityName"`
	CountrySubDivisionCode string `json:"CountrySubDivisionCode"`
	CountryCode          string `json:"CountryCode"`
}

type usaJobsRemuneration struct {
	MinimumRange    string `json:"MinimumRange"`
	MaximumRange    string `json:"MaximumRange"`
	RateIntervalCode string `json:"RateIntervalCode"`
	Description     string `json:"Description"`
}

type userArea struct {
	Details *userDetails `json:"Details,omitempty"`
}

type userDetails struct {
	JobSummary  string   `json:"JobSummary"`
	MajorDuties []string `json:"MajorDuties,omitempty"`
}

// Scrape fetches jobs from the USAJobs API.
func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{
		"site":           s.SiteName(),
		"results_wanted": input.ResultsWanted,
		"search_term":    input.SearchTerm,
	})

	// USAJobs requires Host, Authorization-Key, and User-Agent headers.
	// The API key and email should be set in the client or env.
	apiKey := ""
	email := ""

	wanted := input.ResultsWanted
	if wanted <= 0 {
		wanted = defaultWanted
	}
	pageSize := wanted
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}

	jobs := make([]model.JobPost, 0, wanted)
	seenIDs := make(map[string]bool)
	page := 1

	for len(jobs) < wanted {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		url := buildURL(s.apiURL, input.SearchTerm, input.Location, input.HoursOld, page, pageSize)

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("usajobs: build request: %w", err)
		}
		req.Header.Set("Host", "data.usajobs.gov")
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", email)
		if apiKey != "" {
			req.Header.Set("Authorization-Key", apiKey)
		}

		resp, err := s.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("usajobs: request: %w", err)
		}

		body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("usajobs: read: %w", err)
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("usajobs: status %d", resp.StatusCode)
		}

		var usaResp usaJobsResponse
		if err := json.Unmarshal(body, &usaResp); err != nil {
			return nil, fmt.Errorf("usajobs: decode: %w", err)
		}

		if usaResp.SearchResult == nil || len(usaResp.SearchResult.SearchResultItems) == 0 {
			break
		}

		for _, item := range usaResp.SearchResult.SearchResultItems {
			if len(jobs) >= wanted {
				break
			}
			if item.MatchedObjectId == "" || seenIDs[item.MatchedObjectId] {
				continue
			}
			seenIDs[item.MatchedObjectId] = true

			job, err := mapJob(item)
			if err != nil {
				continue
			}
			jobs = append(jobs, job)
		}

		if len(usaResp.SearchResult.SearchResultItems) < pageSize {
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

func buildURL(baseURL, searchTerm, location string, hoursOld, page, pageSize int) string {
	params := make([]string, 0, 5)
	params = append(params, "ResultsPerPage="+strconv.Itoa(pageSize))
	params = append(params, "Page="+strconv.Itoa(page))
	params = append(params, "Fields=Full")
	if strings.TrimSpace(searchTerm) != "" {
		params = append(params, "Keyword="+strings.ReplaceAll(searchTerm, " ", "+"))
	}
	if strings.TrimSpace(location) != "" {
		params = append(params, "LocationName="+strings.ReplaceAll(location, " ", "+"))
	}
	if hoursOld > 0 {
		daysOld := (hoursOld + 23) / 24 // ceil
		params = append(params, "DatePosted="+strconv.Itoa(daysOld))
	}
	return baseURL + "?" + strings.Join(params, "&")
}

func mapJob(item usaJobsItem) (model.JobPost, error) {
	desc := item.MatchedObjectDescriptor
	if desc == nil {
		return model.JobPost{}, fmt.Errorf("no descriptor")
	}

	title := strings.TrimSpace(desc.PositionTitle)
	jobURL := strings.TrimSpace(desc.PositionURI)
	if title == "" || jobURL == "" {
		return model.JobPost{}, fmt.Errorf("missing title or URL")
	}

	// Build description
	descParts := make([]string, 0, 3)
	if desc.UserArea != nil && desc.UserArea.Details != nil {
		if ds := strings.TrimSpace(desc.UserArea.Details.JobSummary); ds != "" {
			descParts = append(descParts, ds)
		}
		if len(desc.UserArea.Details.MajorDuties) > 0 {
			duties := make([]string, len(desc.UserArea.Details.MajorDuties))
			for i, d := range desc.UserArea.Details.MajorDuties {
				duties[i] = "- " + strings.TrimSpace(d)
			}
			descParts = append(descParts, "Major Duties:\n"+strings.Join(duties, "\n"))
		}
	}
	if qs := strings.TrimSpace(desc.QualificationSummary); qs != "" {
		descParts = append(descParts, qs)
	}

	// Location
	loc := model.Location{}
	if len(desc.PositionLocation) > 0 {
		loc.City = desc.PositionLocation[0].CityName
		loc.State = desc.PositionLocation[0].CountrySubDivisionCode
		loc.Country = desc.PositionLocation[0].CountryCode
	}

	// Compensation
	var comp *model.Compensation
	if len(desc.PositionRemuneration) > 0 {
		comp = mapCompensation(desc.PositionRemuneration[0])
	}

	// Date
	var datePosted *time.Time
	if dp := strings.TrimSpace(desc.PublicationStartDate); dp != "" {
		if t, err := time.Parse("2006-01-02T15:04:05", dp); err == nil {
			datePosted = &t
		} else if t, err := time.Parse(time.RFC3339, dp); err == nil {
			datePosted = &t
		} else if t, err := time.Parse("2006-01-02", dp[:10]); err == nil {
			datePosted = &t
		}
	}

	description := strings.Join(descParts, "\n\n")

	return model.JobPost{
		ID:          "usajobs-" + item.MatchedObjectId,
		Title:       title,
		CompanyName: strings.TrimSpace(desc.OrganizationName),
		JobURL:      jobURL,
		Location:    loc,
		Description: description,
		Compensation: comp,
		DatePosted:  datePosted,
		Site:        string(model.SiteUSAJobs),
		ApplyMethod: "external_url",
	}, nil
}

func mapCompensation(r usaJobsRemuneration) *model.Compensation {
	minVal, minErr := strconv.ParseFloat(r.MinimumRange, 64)
	maxVal, maxErr := strconv.ParseFloat(r.MaximumRange, 64)
	if minErr != nil && maxErr != nil {
		return nil
	}

	interval := model.IntervalYearly
	switch {
	case strings.Contains(r.Description, "Hour") || strings.Contains(r.RateIntervalCode, "Hour"):
		interval = model.IntervalHourly
	case strings.Contains(r.Description, "Month") || strings.Contains(r.RateIntervalCode, "Month"):
		interval = model.IntervalMonthly
	case strings.Contains(r.Description, "Week") || strings.Contains(r.RateIntervalCode, "Week"):
		interval = model.IntervalWeekly
	case strings.Contains(r.Description, "Day") || strings.Contains(r.RateIntervalCode, "Day"):
		interval = model.IntervalDaily
	}

	c := &model.Compensation{
		Interval: interval,
		Currency: "USD",
	}
	if minErr == nil {
		c.MinAmount = &minVal
	}
	if maxErr == nil {
		c.MaxAmount = &maxVal
	}
	return c
}
