package echojobs

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

const apiURL = "https://echojobs.io/api/jobs"

// Scraper fetches jobs from the EchoJobs public API.
type Scraper struct {
	client *http.Client
	apiURL string
}

// New creates a new EchoJobs scraper. If client is nil a default one is used.
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
func (s *Scraper) SiteName() model.Site { return model.SiteEchoJobs }

// Scrape fetches jobs from EchoJobs. The API returns all jobs in one response.
// It supports two response formats: bare JSON array or {"jobs":[...]} wrapper.
func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{
		"site":           s.SiteName(),
		"results_wanted": input.ResultsWanted,
		"search_term":    input.SearchTerm,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("echojobs: build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("echojobs: request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("echojobs: status %d — try using --proxy with a residential proxy", resp.StatusCode)
	}

	body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("echojobs: read: %w", err)
	}

	rawJobs, err := decodeJobs(body)
	if err != nil {
		return nil, fmt.Errorf("echojobs: decode: %w", err)
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

		// Client-side title search
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
			Site: string(s.SiteName()),
		}

		job.ID = buildID(j["id"])
		job.Title = strings.TrimSpace(title)
		job.CompanyName = strings.TrimSpace(toString(j["company"]))
		job.CompanyLogo = strings.TrimSpace(toString(j["company_logo"]))
		job.JobURL = strings.TrimSpace(toString(j["url"]))
		job.Description = strings.TrimSpace(toString(j["description"]))
		job.JobType = strings.TrimSpace(toString(j["job_type"]))

		if loc := strings.TrimSpace(toString(j["location"])); loc != "" {
			job.Location = model.Location{City: loc}
		}

		// Remote detection
		if isRemote, ok := j["is_remote"].(bool); ok {
			job.IsRemote = isRemote
		} else if remote, ok := j["remote"].(bool); ok {
			job.IsRemote = remote
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

		// DatePosted from date_posted or published_at
		job.DatePosted = parseDate(j, "date_posted")
		if job.DatePosted == nil {
			job.DatePosted = parseDate(j, "published_at")
		}

		// Compensation
		job.Compensation = parseCompensation(j)

		out = append(out, job)
	}

	util.Debug("scraper_done", map[string]any{
		"site": s.SiteName(),
		"jobs": len(out),
	})

	if !util.HasMeaningfulJobs(out) {
		return nil, fmt.Errorf("echojobs: no parseable jobs")
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

// buildID creates a deterministic ID using the "echojobs-" prefix.
func buildID(raw any) string {
	switch v := raw.(type) {
	case string:
		if strings.TrimSpace(v) != "" {
			return "echojobs-" + strings.TrimSpace(v)
		}
	case float64:
		return "echojobs-" + strconv.Itoa(int(v))
	case int64:
		return "echojobs-" + strconv.Itoa(int(v))
	case int:
		return "echojobs-" + strconv.Itoa(v)
	case json.Number:
		return "echojobs-" + v.String()
	}
	return ""
}

// parseCompensation extracts salary data from the raw job map.
func parseCompensation(j map[string]any) *model.Compensation {
	var minF, maxF float64
	hasMin, hasMax := false, false

	if minRaw, ok := j["salary_min"]; ok {
		switch v := minRaw.(type) {
		case float64:
			minF, hasMin = v, true
		case int64:
			minF, hasMin = float64(v), true
		case json.Number:
			if f, err := v.Float64(); err == nil {
				minF, hasMin = f, true
			}
		}
	}
	if maxRaw, ok := j["salary_max"]; ok {
		switch v := maxRaw.(type) {
		case float64:
			maxF, hasMax = v, true
		case int64:
			maxF, hasMax = float64(v), true
		case json.Number:
			if f, err := v.Float64(); err == nil {
				maxF, hasMax = f, true
			}
		}
	}

	if !hasMin && !hasMax {
		return nil
	}

	curr := toString(j["salary_currency"])
	if curr == "" {
		curr = "USD"
	}

	return &model.Compensation{
		Interval:  model.IntervalYearly,
		MinAmount: &minF,
		MaxAmount: &maxF,
		Currency:  curr,
	}
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
		time.RFC3339,
		"2006-01-02T15:04:05Z",
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
