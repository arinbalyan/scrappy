package jobtechdev

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/util"
)

const apiURL = "https://jobsearch.api.jobtechdev.se/search"
const maxLimit = 100

// --- API response types ---

type apiResponse struct {
	Total apiTotal `json:"total"`
	Hits  []apiHit `json:"hits"`
}

type apiTotal struct {
	Value int `json:"value"`
}

type apiHit struct {
	ID                 string            `json:"id"`
	Headline           string            `json:"headline"`
	Description        *apiDesc          `json:"description"`
	EmploymentType     *apiLabel         `json:"employment_type"`
	WorkingHoursType   *apiLabel         `json:"working_hours_type"`
	Employer           *apiEmployer      `json:"employer"`
	WorkplaceAddress   *apiAddress       `json:"workplace_address"`
	ApplicationDetails *apiApplication   `json:"application_details"`
	PublicationDate    *string           `json:"publication_date"`
	LastPublicationDate *string          `json:"last_publication_date"`
	WebpageURL         *string           `json:"webpage_url"`
	LogoURL            *string           `json:"logo_url"`
	SalaryDescription  *string           `json:"salary_description"`
	ScopeOfWork        *apiScope         `json:"scope_of_work"`
}

type apiDesc struct {
	Text string `json:"text"`
}

type apiLabel struct {
	Label string `json:"label"`
}

type apiEmployer struct {
	Name string  `json:"name"`
	URL  *string `json:"url"`
}

type apiAddress struct {
	Municipality *string `json:"municipality"`
	Region       *string `json:"region"`
	Country      *string `json:"country"`
}

type apiApplication struct {
	URL   *string `json:"url"`
	Email *string `json:"email"`
}

type apiScope struct {
	Min *float64 `json:"min"`
	Max *float64 `json:"max"`
}

// Scraper fetches jobs from the JobTechDev (Swedish government) API.
type Scraper struct {
	client  *http.Client
	baseURL string
	apiKey  string
}

// New creates a new JobTechDev scraper. The API key is read from the
// JOBTECHDEV_API_KEY environment variable.
func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Timeout: 20 * time.Second})
	}
	return &Scraper{
		client:  client,
		baseURL: apiURL,
		apiKey:  os.Getenv("JOBTECHDEV_API_KEY"),
	}
}

// NewWithURLs creates a JobTechDev scraper with explicit configuration. Used for
// testing to point at a local test server with a known API key.
func NewWithURLs(client *http.Client, baseURL, apiKey string) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Timeout: 20 * time.Second})
	}
	return &Scraper{
		client:  client,
		baseURL: baseURL,
		apiKey:  apiKey,
	}
}

// SiteName returns the site identifier.
func (s *Scraper) SiteName() model.Site { return model.SiteJobTechDev }

// Scrape fetches jobs from the JobTechDev API. It uses offset-based pagination
// with a maximum of 100 results per page. The API key is sent via the api-key
// header. Search is server-side via the q parameter.
func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{
		"site":           s.SiteName(),
		"results_wanted": input.ResultsWanted,
		"search_term":    input.SearchTerm,
	})

	if s.apiKey == "" {
		return nil, fmt.Errorf("jobtechdev: JOBTECHDEV_API_KEY not set")
	}

	wanted := input.ResultsWanted
	if wanted <= 0 {
		wanted = 25
	}

	// Build the search query from search term + location.
	var searchParts []string
	if v := strings.TrimSpace(input.SearchTerm); v != "" {
		searchParts = append(searchParts, v)
	}
	if v := strings.TrimSpace(input.Location); v != "" {
		searchParts = append(searchParts, v)
	}
	query := strings.Join(searchParts, " ")

	jobs := make([]model.JobPost, 0, wanted)
	offset := 0

	for len(jobs) < wanted {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		limit := wanted - len(jobs)
		if limit > maxLimit {
			limit = maxLimit
		}

		// Build the request URL with query parameters.
		u, _ := url.Parse(s.baseURL)
		q := u.Query()
		if query != "" {
			q.Set("q", query)
		}
		q.Set("offset", strconv.Itoa(offset))
		q.Set("limit", strconv.Itoa(limit))
		u.RawQuery = q.Encode()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
		if err != nil {
			return nil, fmt.Errorf("jobtechdev: build request: %w", err)
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("api-key", s.apiKey)

		resp, err := s.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("jobtechdev: request: %w", err)
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			resp.Body.Close()
			return nil, fmt.Errorf("jobtechdev: status %d", resp.StatusCode)
		}

		body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("jobtechdev: read: %w", err)
		}

		var parsed apiResponse
		if err := json.Unmarshal(body, &parsed); err != nil {
			return nil, fmt.Errorf("jobtechdev: decode: %w", err)
		}

		if len(parsed.Hits) == 0 {
			break
		}

		resultsInBatch := 0
		for _, hit := range parsed.Hits {
			if len(jobs) >= wanted {
				break
			}
			job := mapHit(hit)
			if strings.TrimSpace(job.Title) == "" {
				continue
			}
			jobs = append(jobs, job)
			resultsInBatch++
		}

		// Stop paginating if we've hit the total available results
		// or no results came back (last page signal).
		if len(jobs) >= parsed.Total.Value || resultsInBatch == 0 {
			break
		}

		offset += limit
	}

	util.Debug("scraper_done", map[string]any{
		"site": s.SiteName(),
		"jobs": len(jobs),
	})

	if !util.HasMeaningfulJobs(jobs) {
		return nil, fmt.Errorf("jobtechdev: no parseable jobs")
	}

	return jobs, nil
}

// mapHit converts a raw JobTechDev API hit to a model.JobPost.
func mapHit(hit apiHit) model.JobPost {
	job := model.JobPost{
		ID:       "jobtechdev-" + hit.ID,
		Title:    strings.TrimSpace(hit.Headline),
		IsRemote: false,
		Site:     string(model.SiteJobTechDev),
	}

	// Employer name
	if hit.Employer != nil {
		job.CompanyName = strings.TrimSpace(hit.Employer.Name)
	}

	// Company logo
	if hit.LogoURL != nil {
		job.CompanyLogo = strings.TrimSpace(*hit.LogoURL)
	}

	// Job URL: prefer webpage_url, fall back to application_details.url
	if hit.WebpageURL != nil && strings.TrimSpace(*hit.WebpageURL) != "" {
		job.JobURL = strings.TrimSpace(*hit.WebpageURL)
	} else if hit.ApplicationDetails != nil && hit.ApplicationDetails.URL != nil {
		job.JobURL = strings.TrimSpace(*hit.ApplicationDetails.URL)
	}

	// Location: municipality -> city, region -> state, country default "Sweden"
	location := model.Location{Country: "Sweden"}
	if hit.WorkplaceAddress != nil {
		if hit.WorkplaceAddress.Municipality != nil {
			location.City = strings.TrimSpace(*hit.WorkplaceAddress.Municipality)
		}
		if hit.WorkplaceAddress.Region != nil {
			location.State = strings.TrimSpace(*hit.WorkplaceAddress.Region)
		}
		if hit.WorkplaceAddress.Country != nil {
			location.Country = strings.TrimSpace(*hit.WorkplaceAddress.Country)
		}
	}
	job.Location = location

	// Description (raw string, no HTML to strip)
	if hit.Description != nil {
		job.Description = strings.TrimSpace(hit.Description.Text)
	}

	// DatePosted from publication_date (ISO 8601 / RFC3339)
	if hit.PublicationDate != nil {
		clean := strings.TrimSpace(*hit.PublicationDate)
		if clean != "" {
			if t, err := time.Parse(time.RFC3339, clean); err == nil {
				job.DatePosted = &t
			}
		}
	}

	// Employment type
	if hit.EmploymentType != nil {
		job.JobType = strings.TrimSpace(hit.EmploymentType.Label)
	}

	return job
}
