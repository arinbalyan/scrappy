package reliefweb

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

const (
	apiURL        = "https://api.reliefweb.int/v1/jobs"
	appName       = "scrappy"
	defaultLimit  = 25
	maxLimit      = 100
)

// apiResponse maps the ReliefWeb API v1 response envelope.
type apiResponse struct {
	Data []jobEntry `json:"data"`
}

// jobEntry maps a single ReliefWeb job entry.
type jobEntry struct {
	ID     string  `json:"id"`
	Score  float64 `json:"score"`
	Href   string  `json:"href"`
	Fields fields  `json:"fields"`
}

// fields maps the ReliefWeb job fields.
type fields struct {
	Title       string           `json:"title"`
	Body        string           `json:"body,omitempty"`
	URL         string           `json:"url,omitempty"`
	Source      []sourceEntry    `json:"source,omitempty"`
	Date        *dateEntry       `json:"date,omitempty"`
	Country     []countryEntry   `json:"country,omitempty"`
	Theme       []namedEntry     `json:"theme,omitempty"`
	Type        []namedEntry     `json:"type,omitempty"`
}

type sourceEntry struct {
	Name string `json:"name"`
}

type dateEntry struct {
	Created string `json:"created,omitempty"`
	Closing string `json:"closing,omitempty"`
}

type countryEntry struct {
	Name string `json:"name"`
	ISO3 string `json:"iso3,omitempty"`
}

type namedEntry struct {
	Name string `json:"name"`
}

// Scraper fetches jobs from the ReliefWeb API.
type Scraper struct {
	client *http.Client
	apiURL string
}

// New creates a new ReliefWeb scraper.
func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Timeout: 20 * time.Second})
	}
	return &Scraper{client: client, apiURL: apiURL}
}

// NewWithAPIURL creates a new scraper with a custom endpoint (used in tests).
func NewWithAPIURL(client *http.Client, endpoint string) *Scraper {
	s := New(client)
	if strings.TrimSpace(endpoint) != "" {
		s.apiURL = endpoint
	}
	return s
}

// SiteName returns the site identifier.
func (s *Scraper) SiteName() model.Site { return model.SiteReliefWeb }

// Scrape fetches jobs from the ReliefWeb API.
func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{
		"site":           s.SiteName(),
		"results_wanted": input.ResultsWanted,
		"search_term":    input.SearchTerm,
	})

	limit := input.ResultsWanted
	if limit <= 0 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}

	// Build query params
	q := fmt.Sprintf("%s?appname=%s&limit=%d&offset=0", s.apiURL, appName, limit)
	fields := []string{"title", "body", "url", "source", "date", "country", "theme", "type"}
	for _, f := range fields {
		q += "&fields[include][]=" + f
	}
	if strings.TrimSpace(input.SearchTerm) != "" {
		q += "&query[value]=" + urlQueryEscape(strings.TrimSpace(input.SearchTerm))
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, q, nil)
	if err != nil {
		return nil, fmt.Errorf("reliefweb: build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "scrappy/0.1.0 (job-aggregator)")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("reliefweb: request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("reliefweb: status %d", resp.StatusCode)
	}

	body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("reliefweb: read: %w", err)
	}

	var apiResp apiResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("reliefweb: decode: %w", err)
	}

	if len(apiResp.Data) == 0 {
		return nil, fmt.Errorf("reliefweb: no jobs in response")
	}

	wanted := input.ResultsWanted
	if wanted <= 0 {
		wanted = len(apiResp.Data)
	}
	if wanted > len(apiResp.Data) {
		wanted = len(apiResp.Data)
	}

	out := make([]model.JobPost, 0, wanted)
	for _, entry := range apiResp.Data {
		if len(out) >= wanted {
			break
		}
		title := strings.TrimSpace(entry.Fields.Title)
		if title == "" {
			continue
		}

		jobURL := strings.TrimSpace(entry.Fields.URL)
		if jobURL == "" {
			jobURL = strings.TrimSpace(entry.Href)
		}

		job := model.JobPost{
			ID:     "reliefweb-" + entry.ID,
			Title:  title,
			JobURL: jobURL,
			Site:   string(s.SiteName()),
		}

		// Company name from source
		if len(entry.Fields.Source) > 0 {
			if name := strings.TrimSpace(entry.Fields.Source[0].Name); name != "" {
				job.CompanyName = name
			}
		}

		// Description from body
		if body := strings.TrimSpace(entry.Fields.Body); body != "" {
			job.Description = body
		}

		// Location from countries
		if len(entry.Fields.Country) > 0 {
			names := make([]string, 0, len(entry.Fields.Country))
			for _, c := range entry.Fields.Country {
				if n := strings.TrimSpace(c.Name); n != "" {
					names = append(names, n)
				}
			}
			if len(names) > 0 {
				job.Location.Country = names[0]
				if len(names) > 1 {
					job.Location.City = strings.Join(names, ", ")
				}
			}
		}

		// DatePosted from created
		if entry.Fields.Date != nil && entry.Fields.Date.Created != "" {
			job.DatePosted = parseDate(entry.Fields.Date.Created)
		}

		out = append(out, job)
	}

	util.Debug("scraper_done", map[string]any{
		"site": s.SiteName(),
		"jobs": len(out),
	})

	if !util.HasMeaningfulJobs(out) {
		return nil, fmt.Errorf("reliefweb: no parseable jobs")
	}
	return out, nil
}

// urlQueryEscape percent-encodes a string for use in URL query parameters.
func urlQueryEscape(s string) string {
	// Simple encoding for common chars; net/url.URL.Query().Encode is heavier
	s = strings.ReplaceAll(s, " ", "%20")
	s = strings.ReplaceAll(s, "&", "%26")
	s = strings.ReplaceAll(s, "=", "%3D")
	s = strings.ReplaceAll(s, "+", "%2B")
	return s
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
