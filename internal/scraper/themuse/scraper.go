package themuse

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

const defaultAPI = "https://www.themuse.com/api/public/jobs"

// --- API response types ---

type themuseResponse struct {
	Page      int          `json:"page"`
	PageCount int          `json:"page_count"`
	Total     int          `json:"total"`
	Results   []themuseJob `json:"results"`
}

type themuseJob struct {
	ID              int              `json:"id"`
	Name            string           `json:"name"`
	Type            string           `json:"type"`
	ShortName       string           `json:"short_name"`
	Company         *themuseCompany  `json:"company"`
	Locations       []themuseLocation `json:"locations"`
	Categories      []themuseCategory `json:"categories"`
	Levels          []themuseLevel    `json:"levels"`
	Refs            themuseRefs       `json:"refs"`
	PublicationDate string           `json:"publication_date"`
	Contents        string           `json:"contents"`
	ModelType       string           `json:"model_type"`
}

type themuseCompany struct {
	ID        int    `json:"id"`
	ShortName string `json:"short_name"`
	Name      string `json:"name"`
}

type themuseLocation struct {
	Name string `json:"name"`
}

type themuseCategory struct {
	Name string `json:"name"`
}

type themuseLevel struct {
	Name      string `json:"name"`
	ShortName string `json:"short_name"`
}

type themuseRefs struct {
	LandingPage string `json:"landing_page"`
}

// Scraper scrapes jobs from The Muse public API.
type Scraper struct {
	client *http.Client
	apiURL string
}

// New creates a TheMuse scraper with the given HTTP client or a default one.
func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Retries: 2, CookieResetEveryN: 200, Timeout: 15 * time.Second})
	}
	return &Scraper{client: client, apiURL: defaultAPI}
}

// NewWithAPIURL creates a scraper pointing at a custom endpoint (for testing).
func NewWithAPIURL(client *http.Client, endpoint string) *Scraper {
	s := New(client)
	if strings.TrimSpace(endpoint) != "" {
		s.apiURL = strings.TrimSpace(endpoint)
	}
	return s
}

// SiteName returns the site identifier.
func (s *Scraper) SiteName() model.Site { return model.SiteTheMuse }

// Scrape fetches and maps jobs from The Muse API.
func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{
		"site":           s.SiteName(),
		"results_wanted": input.ResultsWanted,
		"search_term":    input.SearchTerm,
		"location":       input.Location,
	})

	// Build request URL with query params
	u, _ := url.Parse(s.apiURL)
	q := u.Query()
	q.Set("page", "0")
	q.Set("descending", "true")
	if strings.TrimSpace(input.Location) != "" {
		q.Set("location", strings.TrimSpace(input.Location))
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("themuse build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/129.0.0.0 Safari/537.36")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("themuse request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("themuse status %d", resp.StatusCode)
	}

	var parsed themuseResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("themuse decode: %w", err)
	}

	if len(parsed.Results) == 0 {
		return nil, fmt.Errorf("themuse no results")
	}

	rawJobs := parsed.Results

	// Client-side search term filtering (TheMuse API has no direct search param)
	if terms := parseSearchTerms(input.SearchTerm); len(terms) > 0 {
		filtered := make([]themuseJob, 0, len(rawJobs))
		for _, j := range rawJobs {
			lower := strings.ToLower(j.Name + " " + j.Contents)
			if matchAny(lower, terms) {
				filtered = append(filtered, j)
			}
		}
		rawJobs = filtered
	}

	limit := input.ResultsWanted
	if limit <= 0 || limit > len(rawJobs) {
		limit = len(rawJobs)
	}

	jobs := make([]model.JobPost, 0, limit)
	for i := 0; i < limit; i++ {
		j := rawJobs[i]
		if j.Name == "" || j.Refs.LandingPage == "" {
			continue
		}

		job := model.JobPost{
			ID:          fmt.Sprintf("themuse-%d", j.ID),
			Title:       j.Name,
			CompanyName: companyName(j.Company),
			JobURL:      j.Refs.LandingPage,
			Description: j.Contents,
			Location:    parseLocation(j.Locations),
		}

		if len(j.Levels) > 0 {
			job.Seniority = j.Levels[0].Name
		}

		if j.PublicationDate != "" {
			job.DatePosted = util.ParseDatePosted(j.PublicationDate)
		}

		jobs = append(jobs, job)
	}

	util.Debug("scraper_done", map[string]any{"site": s.SiteName(), "jobs": len(jobs)})

	if !util.HasMeaningfulJobs(jobs) {
		return nil, fmt.Errorf("themuse no parseable jobs")
	}
	return jobs, nil
}

// companyName safely extracts the company name from a nullable company pointer.
func companyName(c *themuseCompany) string {
	if c == nil {
		return ""
	}
	return c.Name
}

// parseLocation converts TheMuse location entries to a model.Location.
// Format: "City, State, Country"
// parseSearchTerms splits a search term on " OR " and returns lowercase terms.
func parseSearchTerms(raw string) []string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return nil
	}
	parts := strings.Split(s, " OR ")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.Trim(strings.TrimSpace(p), `"`)
		if p != "" {
			out = append(out, strings.ToLower(p))
		}
	}
	return out
}

// matchAny returns true if the haystack contains any of the terms.
func matchAny(hay string, terms []string) bool {
	for _, t := range terms {
		if strings.Contains(hay, t) {
			return true
		}
	}
	return false
}

func parseLocation(locations []themuseLocation) model.Location {
	if len(locations) == 0 {
		return model.Location{}
	}
	raw := strings.TrimSpace(locations[0].Name)
	if raw == "" {
		return model.Location{}
	}
	parts := strings.SplitN(raw, ",", 3)
	loc := model.Location{}
	if len(parts) > 0 {
		loc.City = strings.TrimSpace(parts[0])
	}
	if len(parts) > 1 {
		loc.State = strings.TrimSpace(parts[1])
	}
	if len(parts) > 2 {
		loc.Country = strings.TrimSpace(parts[2])
	}
	return loc
}
