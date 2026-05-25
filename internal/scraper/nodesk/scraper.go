package nodesk

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

const apiURL = "https://nodesk.co/api/jobs/"

// Scraper fetches jobs from the NoDesk public API.
type Scraper struct {
	client *http.Client
	apiURL string
}

// New creates a new NoDesk scraper. If client is nil a default one is used.
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
func (s *Scraper) SiteName() model.Site { return model.SiteNoDesk }

// Scrape fetches jobs from NoDesk. The API returns all jobs in one response.
// It supports two response formats: bare JSON array or {"jobs":[...]} wrapper.
// NoDesk is a remote-only board — IsRemote is always hardcoded to true.
func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{
		"site":           s.SiteName(),
		"results_wanted": input.ResultsWanted,
		"search_term":    input.SearchTerm,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("nodesk: build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("nodesk: request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("nodesk: status %d", resp.StatusCode)
	}

	body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("nodesk: read: %w", err)
	}

	rawJobs, err := decodeJobs(body)
	if err != nil {
		return nil, fmt.Errorf("nodesk: decode: %w", err)
	}

	term := strings.ToLower(strings.TrimSpace(input.SearchTerm))
	wanted := input.ResultsWanted
	if wanted <= 0 {
		wanted = len(rawJobs)
	}
	if wanted > len(rawJobs) {
		wanted = len(rawJobs)
	}

	out := make([]model.JobPost, 0, wanted)
	for _, j := range rawJobs {
		if len(out) >= wanted {
			break
		}
		title, _ := j["title"].(string)
		if strings.TrimSpace(title) == "" {
			continue
		}

		// Client-side title + tag search
		if term != "" {
			hay := strings.ToLower(title)
			if tags, ok := j["tags"].([]any); ok {
				for _, t := range tags {
					if s, ok := t.(string); ok {
						hay += " " + strings.ToLower(s)
					}
				}
			}
			if !strings.Contains(hay, term) {
				continue
			}
		}

		job := model.JobPost{
			Site:     string(s.SiteName()),
			IsRemote: true, // NoDesk is a remote-only board — always true
		}

		job.ID = buildID(j["id"])
		job.Title = strings.TrimSpace(title)
		job.CompanyName = strings.TrimSpace(toString(j["company"]))
		job.CompanyLogoURL = strings.TrimSpace(toString(j["company_logo"]))
		job.JobURL = strings.TrimSpace(toString(j["url"]))
		job.Description = strings.TrimSpace(toString(j["description"]))

		// Location
		if loc := strings.TrimSpace(toString(j["location"])); loc != "" {
			job.Location = model.Location{City: loc}
		}

		// Skills from tags
		if tags, ok := j["tags"].([]any); ok {
			skills := make([]string, 0, len(tags))
			for _, t := range tags {
				if s, ok := t.(string); ok && strings.TrimSpace(s) != "" {
					skills = append(skills, strings.TrimSpace(s))
				}
			}
			job.Skills = skills
		}

		// DatePosted from published_at or date
		job.DatePosted = parseDate(j, "published_at")
		if job.DatePosted == nil {
			job.DatePosted = parseDate(j, "date")
		}

		out = append(out, job)
	}

	util.Debug("scraper_done", map[string]any{
		"site": s.SiteName(),
		"jobs": len(out),
	})

	if !util.HasMeaningfulJobs(out) {
		return nil, fmt.Errorf("nodesk: no parseable jobs")
	}
	return out, nil
}

// decodeJobs tries to decode the response as a JSON array first. If that fails,
// it tries {"jobs":[...]} or {"data":[...]} wrapper objects.
func decodeJobs(body []byte) ([]map[string]any, error) {
	// Try bare array first
	var arr []map[string]any
	if err := json.Unmarshal(body, &arr); err == nil {
		return arr, nil
	}

	// Try object wrapper
	var obj map[string]any
	if err := json.Unmarshal(body, &obj); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	for _, key := range []string{"jobs", "data"} {
		if raw, ok := obj[key]; ok {
			switch v := raw.(type) {
			case []any:
				result := make([]map[string]any, 0, len(v))
				for _, item := range v {
					if m, ok := item.(map[string]any); ok {
						result = append(result, m)
					}
				}
				if len(result) > 0 {
					return result, nil
				}
			}
		}
	}
	return nil, fmt.Errorf("no array or jobs/data field found")
}

// buildID creates a deterministic ID using the "nodesk-" prefix.
func buildID(raw any) string {
	switch v := raw.(type) {
	case string:
		if strings.TrimSpace(v) != "" {
			return "nodesk-" + strings.TrimSpace(v)
		}
	case float64:
		return "nodesk-" + strconv.Itoa(int(v))
	case int64:
		return "nodesk-" + strconv.Itoa(int(v))
	case int:
		return "nodesk-" + strconv.Itoa(v)
	case json.Number:
		return "nodesk-" + v.String()
	}
	return ""
}

// parseDate extracts and parses an ISO date string from the job map.
func parseDate(j map[string]any, key string) *time.Time {
	raw, ok := j[key]
	if !ok {
		return nil
	}
	s, ok := raw.(string)
	if !ok || strings.TrimSpace(s) == "" {
		return nil
	}
	s = strings.TrimSpace(s)
	formats := []string{
		"2006-01-02T15:04:05Z",
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02",
	}
	for _, f := range formats {
		t, err := time.Parse(f, s)
		if err == nil {
			return &t
		}
	}
	return nil
}

// toString safely extracts a string value from a map.
func toString(v any) string {
	if v == nil {
		return ""
	}
	switch s := v.(type) {
	case string:
		return s
	default:
		return fmt.Sprintf("%v", v)
	}
}
