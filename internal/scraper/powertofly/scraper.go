package powertofly

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

const apiURL = "https://powertofly.com/jobs/rss"

// apiResponse maps the JSON shape returned by the PowerToFly API.
type apiResponse struct {
	Items  []item `json:"items"`
	Status string `json:"status,omitempty"`
}

// item maps a single PowerToFly job item.
type item struct {
	Title       string   `json:"title,omitempty"`
	Description string   `json:"description,omitempty"`
	Link        string   `json:"link,omitempty"`
	JobLocation string   `json:"job_location,omitempty"`
	PublishedOn string   `json:"published_on,omitempty"`
	Categories  []string `json:"categories,omitempty"`
	Type        string   `json:"type,omitempty"`
	GUID        string   `json:"guid,omitempty"`
}

// Scraper fetches jobs from PowerToFly.
type Scraper struct {
	client *http.Client
	apiURL string
}

// New creates a new PowerToFly scraper.
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
func (s *Scraper) SiteName() model.Site { return model.SitePowerToFly }

// Scrape fetches jobs from PowerToFly.
func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{
		"site":           s.SiteName(),
		"results_wanted": input.ResultsWanted,
		"search_term":    input.SearchTerm,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("powertofly: build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("powertofly: request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("powertofly: status %d", resp.StatusCode)
	}

	body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("powertofly: read: %w", err)
	}

	var data apiResponse
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("powertofly: decode: %w", err)
	}

	if len(data.Items) == 0 {
		return nil, fmt.Errorf("powertofly: no items in response")
	}

	term := strings.ToLower(strings.TrimSpace(input.SearchTerm))
	wanted := input.ResultsWanted
	if wanted <= 0 {
		wanted = len(data.Items)
	}
	if wanted > len(data.Items) {
		wanted = len(data.Items)
	}

	out := make([]model.JobPost, 0, wanted)
	for _, item := range data.Items {
		if len(out) >= wanted {
			break
		}
		title := strings.TrimSpace(item.Title)
		link := strings.TrimSpace(item.Link)
		if title == "" || link == "" {
			continue
		}

		// Client-side search term filtering
		if term != "" {
			hay := strings.ToLower(title + " " + item.Description + " " + item.JobLocation + " " + strings.Join(item.Categories, " "))
			if !strings.Contains(hay, term) {
				continue
			}
		}

		job := model.JobPost{
			ID:     "powertofly-" + extractID(item.GUID, link),
			Title:  title,
			JobURL: link,
			Site:   string(s.SiteName()),
		}

		// Company name from description field (PowerToFly-specific)
		if desc := strings.TrimSpace(item.Description); desc != "" {
			job.CompanyName = desc
		}

		// Remote detection
		if strings.ToLower(strings.TrimSpace(item.Type)) == "remote" {
			job.IsRemote = true
		}

		// Location
		if loc := strings.TrimSpace(item.JobLocation); loc != "" {
			parts := strings.SplitN(loc, ",", 2)
			job.Location.City = strings.TrimSpace(parts[0])
			if len(parts) > 1 {
				job.Location.Country = strings.TrimSpace(parts[1])
			}
		}

		// Build description from categories and type
		descParts := make([]string, 0, 4)
		if len(item.Categories) > 0 {
			descParts = append(descParts, "Department: "+item.Categories[0])
			if len(item.Categories) > 1 {
				descParts = append(descParts, "Categories: "+strings.Join(item.Categories, ", "))
			}
		}
		if item.Type != "" {
			descParts = append(descParts, "Type: "+item.Type)
		}
		if item.JobLocation != "" {
			descParts = append(descParts, "Location: "+item.JobLocation)
		}
		if len(descParts) > 0 {
			job.Description = strings.Join(descParts, "\n")
		}

		// Parse date
		if pd := strings.TrimSpace(item.PublishedOn); pd != "" {
			job.DatePosted = parseDate(pd)
		}

		out = append(out, job)
	}

	util.Debug("scraper_done", map[string]any{
		"site": s.SiteName(),
		"jobs": len(out),
	})

	if !util.HasMeaningfulJobs(out) {
		return nil, fmt.Errorf("powertofly: no parseable jobs")
	}
	return out, nil
}

// extractID extracts a short ID from a URL by using the last path segment.
func extractID(guid, link string) string {
	u := guid
	if u == "" {
		u = link
	}
	u = strings.TrimRight(u, "/")
	parts := strings.Split(u, "/")
	for i := len(parts) - 1; i >= 0; i-- {
		if p := strings.TrimSpace(parts[i]); p != "" {
			return p
		}
	}
	return "unknown"
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
		time.RFC822Z,
		time.RFC822,
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
