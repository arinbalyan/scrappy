package talroo

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/util"
)

const (
	apiURL       = "https://api.jobs2careers.com/api/search.php"
	defaultLimit = 25
	maxLimit     = 200
)

// apiResponse maps the Talroo API response.
type apiResponse struct {
	Total int        `json:"total"`
	Start int        `json:"start"`
	Count int        `json:"count"`
	Jobs  []talrooJob `json:"jobs"`
}

// talrooJob maps a single Talroo job entry.
type talrooJob struct {
	Title       string   `json:"title"`
	Date        string   `json:"date"`
	Onclick     string   `json:"onclick"`
	Company     string   `json:"company"`
	City        []string `json:"city"`
	Coordinates []string `json:"coordinates"`
	Description string   `json:"description"`
}

// Scraper fetches jobs from the Talroo API.
type Scraper struct {
	client        *http.Client
	apiURL        string
	publisherID   string
	publisherPass string
}

// New creates a new Talroo scraper. Credentials are read from environment
// variables TALROO_PUBLISHER_ID and TALROO_PUBLISHER_PASS. If either is
// missing, Scrape returns an error.
func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Timeout: 20 * time.Second})
	}
	return &Scraper{
		client:        client,
		apiURL:        apiURL,
		publisherID:   os.Getenv("TALROO_PUBLISHER_ID"),
		publisherPass: os.Getenv("TALROO_PUBLISHER_PASS"),
	}
}

// NewWithAPIURL creates a new scraper with a custom endpoint (used in tests).
func NewWithAPIURL(client *http.Client, endpoint, publisherID, publisherPass string) *Scraper {
	s := &Scraper{
		client:        client,
		apiURL:        apiURL,
		publisherID:   publisherID,
		publisherPass: publisherPass,
	}
	if client == nil {
		s.client = util.NewHTTPClient(util.ClientOptions{Timeout: 20 * time.Second})
	}
	if strings.TrimSpace(endpoint) != "" {
		s.apiURL = endpoint
	}
	return s
}

// SiteName returns the site identifier.
func (s *Scraper) SiteName() model.Site { return model.SiteTalroo }

// Scrape fetches jobs from the Talroo API.
func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{
		"site":           s.SiteName(),
		"results_wanted": input.ResultsWanted,
		"search_term":    input.SearchTerm,
	})

	if s.publisherID == "" || s.publisherPass == "" {
		return nil, fmt.Errorf("talroo: TALROO_PUBLISHER_ID and TALROO_PUBLISHER_PASS must be set")
	}

	limit := input.ResultsWanted
	if limit <= 0 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}

	// Build query params
	params := fmt.Sprintf(
		"id=%s&pass=%s&format=json&full_desc=1&ip=127.0.0.1&limit=%d",
		s.publisherID, s.publisherPass, limit,
	)
	if strings.TrimSpace(input.SearchTerm) != "" {
		params += "&q=" + urlQueryEscape(strings.TrimSpace(input.SearchTerm))
	}
	if strings.TrimSpace(input.Location) != "" {
		params += "&l=" + urlQueryEscape(strings.TrimSpace(input.Location))
	}

	requestURL := s.apiURL + "?" + params

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("talroo: build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("talroo: request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("talroo: status %d", resp.StatusCode)
	}

	body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("talroo: read: %w", err)
	}

	var apiResp apiResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("talroo: decode: %w", err)
	}

	if len(apiResp.Jobs) == 0 {
		return nil, fmt.Errorf("talroo: no jobs in response")
	}

	wanted := input.ResultsWanted
	if wanted <= 0 {
		wanted = len(apiResp.Jobs)
	}
	if wanted > len(apiResp.Jobs) {
		wanted = len(apiResp.Jobs)
	}

	out := make([]model.JobPost, 0, wanted)
	seenIDs := make(map[string]bool)

	for _, raw := range apiResp.Jobs {
		if len(out) >= wanted {
			break
		}

		onclick := strings.TrimSpace(raw.Onclick)
		if onclick == "" {
			continue
		}
		title := strings.TrimSpace(raw.Title)
		if title == "" {
			continue
		}

		jobID := "talroo-" + onclick
		if seenIDs[jobID] {
			continue
		}
		seenIDs[jobID] = true

		job := model.JobPost{
			ID:      jobID,
			Title:   title,
			JobURL:  onclick,
			Site:    string(s.SiteName()),
		}

		// Company name
		if company := strings.TrimSpace(raw.Company); company != "" {
			job.CompanyName = company
		}

		// Description
		if desc := strings.TrimSpace(raw.Description); desc != "" {
			job.Description = desc
		}

		// Location from city array
		if len(raw.City) > 0 {
			cityStr := strings.Join(raw.City, ", ")
			job.Location.City = cityStr

			// Remote detection based on city
			for _, c := range raw.City {
				if strings.Contains(strings.ToLower(c), "remote") {
					job.IsRemote = true
					break
				}
			}
		}

		// DatePosted
		if d := strings.TrimSpace(raw.Date); d != "" {
			job.DatePosted = parseDate(d)
		}

		out = append(out, job)
	}

	util.Debug("scraper_done", map[string]any{
		"site": s.SiteName(),
		"jobs": len(out),
	})

	if !util.HasMeaningfulJobs(out) {
		return nil, fmt.Errorf("talroo: no parseable jobs")
	}
	return out, nil
}

// urlQueryEscape percent-encodes a string for use in URL query parameters.
func urlQueryEscape(s string) string {
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
		"Mon, 02 Jan 2006 15:04:05 -0700",
		"Mon, 02 Jan 2006 15:04:05 MST",
	}
	for _, f := range formats {
		t, err := time.Parse(f, v)
		if err == nil {
			return &t
		}
	}
	return nil
}
