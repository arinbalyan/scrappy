package exa

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
	defaultNumResults = 15
	apiURL            = "https://api.exa.ai"
)

var defaultJobDomains = []string{
	"linkedin.com",
	"indeed.com",
	"glassdoor.com",
	"lever.co",
	"greenhouse.io",
	"jobs.ashbyhq.com",
	"boards.greenhouse.io",
	"apply.workable.com",
	"angel.co",
	"wellfound.com",
	"remoteok.com",
	"weworkremotely.com",
	"stackoverflow.com",
	"dice.com",
	"ziprecruiter.com",
	"simplyhired.com",
	"monster.com",
	"careers.google.com",
}

// Scraper fetches jobs via the Exa.ai API.
type Scraper struct {
	client *http.Client
	apiURL string
	apiKey string
}

// New creates a new Exa scraper. Reads EXA_API_KEY from the environment.
func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{
			Timeout:  30 * time.Second,
			Retries:  1,
		})
	}
	return &Scraper{
		client: client,
		apiURL: apiURL,
	}
}

// NewWithAPIURL creates a scraper with a custom endpoint (used in tests).
func NewWithAPIURL(client *http.Client, endpoint string) *Scraper {
	s := New(client)
	if strings.TrimSpace(endpoint) != "" {
		s.apiURL = strings.TrimSpace(endpoint)
	}
	return s
}

// NewWithAPIKey creates a scraper with a specific API key.
func NewWithAPIKey(client *http.Client, apiKey string) *Scraper {
	s := New(client)
	if strings.TrimSpace(apiKey) != "" {
		s.apiKey = strings.TrimSpace(apiKey)
	}
	return s
}

// SiteName returns the site identifier.
func (s *Scraper) SiteName() model.Site { return model.SiteExa }

// --- API request/response types ---

type exaRequest struct {
	Query              string   `json:"query"`
	NumResults         int      `json:"numResults"`
	Type               string   `json:"type"`
	IncludeDomains     []string `json:"includeDomains"`
	Text               bool     `json:"text"`
	Summary            bool     `json:"summary"`
	StartPublishedDate string   `json:"startPublishedDate,omitempty"`
}

type exaResponse struct {
	Results []exaResult `json:"results"`
}

type exaResult struct {
	URL           string `json:"url"`
	Title         string `json:"title"`
	Author        string `json:"author,omitempty"`
	Text          string `json:"text,omitempty"`
	Summary       string `json:"summary,omitempty"`
	PublishedDate string `json:"publishedDate,omitempty"`
}

// Scrape fetches jobs from the Exa.ai API.
func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{
		"site":           s.SiteName(),
		"results_wanted": input.ResultsWanted,
		"search_term":    input.SearchTerm,
	})

	wanted := input.ResultsWanted
	if wanted <= 0 {
		wanted = defaultNumResults
	}

	// Build the query
	queryParts := []string{}
	if input.SearchTerm != "" {
		queryParts = append(queryParts, input.SearchTerm)
	}
	if input.Location != "" {
		queryParts = append(queryParts, "in "+input.Location)
	}
	if input.IsRemote {
		queryParts = append(queryParts, "remote")
	}
	queryParts = append(queryParts, "job posting")
	query := strings.Join(queryParts, " ")

	// Build request body
	body := exaRequest{
		Query:          query,
		NumResults:     wanted,
		Type:           "auto",
		IncludeDomains: defaultJobDomains,
		Text:           true,
		Summary:        true,
	}
	if input.HoursOld > 0 {
		since := time.Now().Add(-time.Duration(input.HoursOld) * time.Hour)
		body.StartPublishedDate = since.Format("2006-01-02")
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("exa: marshal: %w", err)
	}

	url := s.apiURL + "/searchAndContents"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(jsonBody)))
	if err != nil {
		return nil, fmt.Errorf("exa: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "EverJobs/1.0")
	if s.apiKey != "" {
		req.Header.Set("x-api-key", s.apiKey)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("exa: request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("exa: status %d", resp.StatusCode)
	}

	respBody, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("exa: read: %w", err)
	}

	var exaResp exaResponse
	if err := json.Unmarshal(respBody, &exaResp); err != nil {
		return nil, fmt.Errorf("exa: decode: %w", err)
	}

	jobs := make([]model.JobPost, 0, wanted)
	for _, result := range exaResp.Results {
		if len(jobs) >= wanted {
			break
		}
		job := processResult(result)
		if job != nil {
			jobs = append(jobs, *job)
		}
	}

	util.Debug("scraper_done", map[string]any{
		"site": s.SiteName(),
		"jobs": len(jobs),
	})

	if !util.HasMeaningfulJobs(jobs) {
		return nil, fmt.Errorf("exa: no parseable jobs")
	}
	return jobs, nil
}

// processResult converts an Exa API result into a JobPost.
func processResult(result exaResult) *model.JobPost {
	if result.URL == "" {
		return nil
	}

	title := result.Title
	if title == "" {
		title = extractTitleFromURL(result.URL)
	}
	if title == "" {
		return nil
	}

	companyName := result.Author
	if companyName == "" {
		companyName = extractCompanyFromURL(result.URL)
	}

	description := result.Text
	if description == "" {
		description = result.Summary
	}

	// Detect remote from title or description
	titleAndDesc := strings.ToLower(title + " " + description)
	isRemote := strings.Contains(titleAndDesc, "remote") ||
		strings.Contains(titleAndDesc, "work from home") ||
		strings.Contains(titleAndDesc, "wfh")

	// Parse date
	var datePosted *time.Time
	if result.PublishedDate != "" {
		if t, err := time.Parse(time.RFC3339, result.PublishedDate); err == nil {
			datePosted = &t
		} else if len(result.PublishedDate) >= 10 {
			if t, err := time.Parse("2006-01-02", result.PublishedDate[:10]); err == nil {
				datePosted = &t
			}
		}
	}

	job := &model.JobPost{
		ID:          "exa-" + hashURL(result.URL),
		Title:       title,
		CompanyName: companyName,
		JobURL:      result.URL,
		Description: description,
		Site:        string(model.SiteExa),
		IsRemote:    isRemote,
		DatePosted:  datePosted,
		ApplyMethod: "external_url",
	}

	return job
}

// extractCompanyFromURL tries to extract a company name from common ATS URL patterns.
func extractCompanyFromURL(url string) string {
	domains := []string{
		"boards.greenhouse.io/",
		"jobs.ashbyhq.com/",
		"jobs.lever.co/",
		".lever.co/",
		"apply.workable.com/",
	}
	for _, domain := range domains {
		if idx := strings.Index(url, domain); idx >= 0 {
			rest := url[idx+len(domain):]
			parts := strings.Split(rest, "/")
			if len(parts) > 0 && strings.TrimSpace(parts[0]) != "" {
				return humanize(parts[0])
			}
		}
	}
	return ""
}

// extractTitleFromURL extracts a title from the URL path as a fallback.
// Ignores domain names (containing dots) and generic path segments.
func extractTitleFromURL(url string) string {
	// Remove query string
	if idx := strings.Index(url, "?"); idx >= 0 {
		url = url[:idx]
	}
	url = strings.TrimRight(url, "/")
	parts := strings.Split(url, "/")
	for i := len(parts) - 1; i >= 0; i-- {
		p := strings.TrimSpace(parts[i])
		if p == "" {
			continue
		}
		// Skip domain-like segments (contain dots) and generic path segments
		if strings.Contains(p, ".") {
			continue
		}
		lower := strings.ToLower(p)
		if lower == "jobs" || lower == "careers" || lower == "job" ||
			lower == "view" || lower == "position" || lower == "apply" ||
			lower == "https" || lower == "http" || lower == "https:" || lower == "http:" {
			continue
		}
		return humanize(p)
	}
	return ""
}

// humanize converts a URL slug to a human-readable string.
func humanize(slug string) string {
	s := strings.ReplaceAll(slug, "-", " ")
	s = strings.ReplaceAll(s, "_", " ")
	words := strings.Fields(s)
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + strings.ToLower(w[1:])
		}
	}
	return strings.Join(words, " ")
}

// hashURL produces a deterministic hash ID from a URL.
func hashURL(url string) string {
	var h int
	for i := 0; i < len(url); i++ {
		h = (h<<5 - h) + int(url[i])
	}
	if h < 0 {
		h = -h
	}
	const charset = "0123456789abcdefghijklmnopqrstuvwxyz"
	result := make([]byte, 0, 8)
	n := h
	for n > 0 && len(result) < 8 {
		result = append(result, charset[n%36])
		n /= 36
	}
	if len(result) == 0 {
		result = append(result, '0')
	}
	return string(result)
}
