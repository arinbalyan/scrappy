package findwork

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/util"
)

const (
	defaultAPIBase = "https://findwork.dev/api/jobs/"
	defaultResults = 25
	rateLimitGap   = 350 * time.Millisecond // ~3 req/s
)

// apiResponse is the top-level FindWork API response envelope.
type apiResponse struct {
	Count    int        `json:"count"`
	Next     *string    `json:"next"`
	Previous *string    `json:"previous"`
	Results  []apiJob   `json:"results"`
}

// apiJob is a single job posting from the FindWork API.
type apiJob struct {
	ID                int      `json:"id"`
	Role              string   `json:"role"`
	CompanyName       string   `json:"company_name"`
	CompanyNumEmployees *string `json:"company_num_employees"`
	EmploymentType    *string  `json:"employment_type"`
	Location          *string  `json:"location"`
	Remote            bool     `json:"remote"`
	Logo              *string  `json:"logo"`
	URL               string   `json:"url"`
	Text              *string  `json:"text"`
	DatePosted        string   `json:"date_posted"`
	Keywords          []string `json:"keywords"`
	Source            string   `json:"source"`
}

// Scraper scrapes jobs from the FindWork API.
type Scraper struct {
	client  *http.Client
	apiBase string
	apiKey  string
}

// New creates a FindWork scraper. The API key is read from the FINDWORK_API_KEY
// environment variable. Register at https://findwork.dev/developers/.
func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Retries: 2, Timeout: 20 * time.Second})
	}
	return &Scraper{
		client:  client,
		apiBase: defaultAPIBase,
		apiKey:  os.Getenv("FINDWORK_API_KEY"),
	}
}

// NewWithURLs creates a FindWork scraper with explicit configuration. Used for
// testing to point at a local test server.
func NewWithURLs(client *http.Client, apiBase, apiKey string) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Retries: 2, Timeout: 20 * time.Second})
	}
	return &Scraper{
		client:  client,
		apiBase: apiBase,
		apiKey:  apiKey,
	}
}

// SiteName returns model.SiteFindwork.
func (s *Scraper) SiteName() model.Site { return model.SiteFindwork }

// Scrape fetches jobs from the FindWork API. It handles cursor-based pagination
// via the next field, deduplicates by job ID, and maps the response to model.JobPost.
func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	if s.apiKey == "" {
		return nil, fmt.Errorf("findwork missing API key: set FINDWORK_API_KEY")
	}

	wanted := input.ResultsWanted
	if wanted <= 0 {
		wanted = defaultResults
	}

	searchTerm := strings.TrimSpace(input.SearchTerm)
	location := strings.TrimSpace(input.Location)

	jobs := make([]model.JobPost, 0, wanted)
	seen := make(map[int]struct{})
	nextURL := s.buildInitialURL(searchTerm, location)
	isFirstPage := true

	for len(jobs) < wanted && nextURL != "" {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		if !isFirstPage {
			// Rate limit: ~3 req/s
			time.Sleep(rateLimitGap)
		}
		isFirstPage = false

		body, err := s.fetchPage(ctx, nextURL)
		if err != nil {
			return nil, fmt.Errorf("findwork fetch: %w", err)
		}

		var parsed apiResponse
		if err := json.Unmarshal(body, &parsed); err != nil {
			return nil, fmt.Errorf("findwork decode: %w", err)
		}

		if len(parsed.Results) == 0 {
			break
		}

		for _, r := range parsed.Results {
			if len(jobs) >= wanted {
				break
			}

			if _, exists := seen[r.ID]; exists {
				continue
			}
			seen[r.ID] = struct{}{}

			job, err := mapJob(r)
			if err != nil {
				continue
			}
			jobs = append(jobs, job)
		}

		if parsed.Next != nil && *parsed.Next != "" {
			nextURL = s.buildNextURL(*parsed.Next)
		} else {
			nextURL = ""
		}
	}

	if !util.HasMeaningfulJobs(jobs) {
		return nil, fmt.Errorf("findwork no parseable jobs")
	}

	if len(jobs) > wanted {
		jobs = jobs[:wanted]
	}

	return jobs, nil
}

// buildInitialURL constructs the first API request URL with query parameters.
func (s *Scraper) buildInitialURL(searchTerm, location string) string {
	if searchTerm == "" && location == "" {
		return s.apiBase
	}
	u, _ := url.Parse(s.apiBase)
	q := u.Query()
	if searchTerm != "" {
		q.Set("search", searchTerm)
	}
	if location != "" {
		q.Set("location", location)
	}
	q.Set("sort_by", "relevance")
	u.RawQuery = q.Encode()
	return u.String()
}

// buildNextURL resolves the next-page URL, handling relative URLs in test
// environments (e.g. httptest) where Scheme/Host are empty.
func (s *Scraper) buildNextURL(next string) string {
	u, err := url.Parse(next)
	if err != nil {
		// If Parse failed, resolve the raw path against the base.
		parsed, perr := url.Parse(s.apiBase)
		if perr != nil {
			return s.apiBase
		}
		ref := &url.URL{Path: next}
		return parsed.ResolveReference(ref).String()
	}
	if u != nil && u.Scheme != "" {
		return next // absolute URL — good as-is
	}
	// Relative URL — resolve against the base.
	parsed, perr := url.Parse(s.apiBase)
	if perr != nil {
		if u != nil {
			return u.String()
		}
		return s.apiBase
	}
	if u != nil {
		return parsed.ResolveReference(u).String()
	}
	return s.apiBase
}

// fetchPage performs an HTTP GET against the given URL with proper auth headers.
func (s *Scraper) fetchPage(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Token "+s.apiKey)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("findwork request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("findwork status %d", resp.StatusCode)
	}

	body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("findwork read: %w", err)
	}
	return body, nil
}

// mapJob converts a raw FindWork API job to a model.JobPost.
func mapJob(raw apiJob) (model.JobPost, error) {
	title := strings.TrimSpace(raw.Role)
	jobURL := strings.TrimSpace(raw.URL)
	if title == "" || jobURL == "" {
		return model.JobPost{}, fmt.Errorf("findwork job missing title or url")
	}

	job := model.JobPost{
		ID:          fmt.Sprintf("findwork-%d", raw.ID),
		Title:       title,
		CompanyName: strings.TrimSpace(raw.CompanyName),
		JobURL:      jobURL,
		IsRemote:    raw.Remote,
	}

	// Location
	if raw.Location != nil {
		loc := strings.TrimSpace(*raw.Location)
		if loc != "" {
			job.Location = model.Location{City: loc}
		}
	}

	// Description (plain text — strip HTML if present)
	if raw.Text != nil {
		desc := strings.TrimSpace(*raw.Text)
		if desc != "" {
			job.Description = stripHTML(desc)
		}
	}

	// Date posted
	if raw.DatePosted != "" {
		if t := util.ParseDatePosted(raw.DatePosted); t != nil {
			job.DatePosted = t
		}
	}

	// Keywords as skills
	if len(raw.Keywords) > 0 {
		job.Skills = raw.Keywords
	}

	return job, nil
}

// stripHTML removes HTML tags from a string and normalises whitespace.
func stripHTML(v string) string {
	if v == "" {
		return ""
	}
	// Simple tag stripping — sufficient for FindWork's HTML fragments
	result := make([]byte, 0, len(v))
	inTag := false
	for i := 0; i < len(v); i++ {
		c := v[i]
		if c == '<' {
			inTag = true
			continue
		}
		if c == '>' {
			inTag = false
			continue
		}
		if !inTag {
			result = append(result, c)
		}
	}
	return strings.Join(strings.Fields(string(result)), " ")
}
