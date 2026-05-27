package jobsch

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/util"
)

const apiURL = "https://www.jobs.ch/api/v1/public/search"

// Scraper fetches jobs from the Jobs.ch API.
type Scraper struct {
	client *http.Client
	apiURL string
}

// New creates a new Jobs.ch scraper.
func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Retries: 2, Timeout: 20 * time.Second})
	}
	return &Scraper{client: client, apiURL: apiURL}
}

// NewWithAPIURL creates a scraper with a custom endpoint (used in tests).
func NewWithAPIURL(client *http.Client, endpoint string) *Scraper {
	s := New(client)
	if strings.TrimSpace(endpoint) != "" {
		s.apiURL = endpoint
	}
	return s
}

// SiteName returns the site identifier.
func (s *Scraper) SiteName() model.Site { return model.SiteJobsCH }

// Scrape fetches jobs from the Jobs.ch API.
func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{
		"site":           s.SiteName(),
		"results_wanted": input.ResultsWanted,
		"search_term":    input.SearchTerm,
	})

	wanted := input.ResultsWanted
	if wanted <= 0 {
		wanted = 25
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("jobsch: build request: %w", err)
	}

	q := req.URL.Query()
	q.Set("rows", fmt.Sprintf("%d", wanted))
	if strings.TrimSpace(input.SearchTerm) != "" {
		q.Set("query", strings.TrimSpace(input.SearchTerm))
	}
	req.URL.RawQuery = q.Encode()

	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("jobsch: request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("jobsch: status %d", resp.StatusCode)
	}

	body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("jobsch: read: %w", err)
	}

	var parsed apiResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("jobsch: decode: %w", err)
	}

	if len(parsed.Documents) == 0 {
		return nil, fmt.Errorf("jobsch: no documents returned")
	}

	limit := wanted
	if limit > len(parsed.Documents) {
		limit = len(parsed.Documents)
	}

	out := make([]model.JobPost, 0, limit)
	for _, doc := range parsed.Documents[:limit] {
		if strings.TrimSpace(doc.Title) == "" || strings.TrimSpace(doc.JobID) == "" {
			continue
		}

		// Build job URL
		jobURL := doc.Links.DetailEN.Href
		if jobURL == "" {
			jobURL = fmt.Sprintf("https://www.jobs.ch/en/vacancies/detail/%s/", doc.JobID)
		}

		job := model.JobPost{
			ID:          "jobsch-" + strings.TrimSpace(doc.JobID),
			Title:       strings.TrimSpace(doc.Title),
			CompanyName: strings.TrimSpace(doc.CompanyName),
			JobURL:      jobURL,
			Description: util.StripHTML(doc.Preview),
			Location:    model.Location{Country: "Switzerland"},
			Site:        string(s.SiteName()),
		}

		// Parse date
		if dp := strings.TrimSpace(doc.PublicationDate); dp != "" {
			job.DatePosted = parseDate(dp)
		}

		out = append(out, job)
	}

	util.Debug("scraper_done", map[string]any{
		"site": s.SiteName(),
		"jobs": len(out),
	})

	if !util.HasMeaningfulJobs(out) {
		return nil, fmt.Errorf("jobsch: no parseable jobs")
	}
	return out, nil
}

type apiResponse struct {
	Documents []apiDocument `json:"documents"`
	NumPages  int           `json:"num_pages"`
}

type apiDocument struct {
	JobID           string `json:"job_id"`
	Title           string `json:"title"`
	CompanyName     string `json:"company_name"`
	Preview         string `json:"preview"`
	PublicationDate string `json:"publication_date"`
	Links           struct {
		DetailEN struct {
			Href string `json:"href"`
		} `json:"detail_en"`
	} `json:"_links"`
}

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
