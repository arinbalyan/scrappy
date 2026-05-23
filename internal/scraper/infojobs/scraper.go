// Package infojobs implements a scraper for InfoJobs (infojobs.com),
// Spain's largest job board. It uses the official InfoJobs API with Basic Auth.
package infojobs

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	internalemail "github.com/arinbalyan/scrappy/internal/email"
	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/util"
)

const defaultAPIURL = "https://api.infojobs.net/api/7/offer"

// Scraper fetches job postings from InfoJobs via its REST API.
type Scraper struct {
	client       *http.Client
	apiURL       string
	clientID     string
	clientSecret string
}

// New creates an InfoJobs scraper. If client is nil, a default is created.
// Credentials are read from INFOJOBS_CLIENT_ID and INFOJOBS_CLIENT_SECRET env vars
// at scrape time (lazy) to support test-time overrides via t.Setenv.
func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Retries: 3, Timeout: 25 * time.Second})
	}
	return &Scraper{
		client: client,
		apiURL: defaultAPIURL,
	}
}

// readEnv is a variable so tests can inject values without t.Setenv.
var readEnv = func(key string) string {
	return os.Getenv(key)
}

// NewWithURLs creates a scraper with a custom API endpoint for testing.
func NewWithURLs(client *http.Client, endpoint string) *Scraper {
	s := New(client)
	if strings.TrimSpace(endpoint) != "" {
		s.apiURL = strings.TrimSpace(endpoint)
	}
	return s
}

// SiteName returns the site identifier.
func (s *Scraper) SiteName() model.Site { return model.SiteInfoJobs }

// Scrape fetches job postings from InfoJobs.
func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	wanted := input.ResultsWanted
	if wanted <= 0 {
		wanted = 25
	}

	// Lazy-read credentials so test-time env overrides (t.Setenv) work correctly.
	clientID := s.clientID
	clientSecret := s.clientSecret
	if clientID == "" {
		clientID = strings.TrimSpace(readEnv("INFOJOBS_CLIENT_ID"))
	}
	if clientSecret == "" {
		clientSecret = strings.TrimSpace(readEnv("INFOJOBS_CLIENT_SECRET"))
	}
	if clientID == "" || clientSecret == "" {
		util.Warn("infojobs_credentials_missing", map[string]any{
			"msg": "INFOJOBS_CLIENT_ID or INFOJOBS_CLIENT_SECRET not set — returning empty results",
		})
		return nil, fmt.Errorf("infojobs: API credentials not configured")
	}

	// Basic auth token: base64(clientID:clientSecret)
	authToken := base64.StdEncoding.EncodeToString([]byte(clientID + ":" + clientSecret))

	jobs := make([]model.JobPost, 0, wanted)
	seenIDs := make(map[string]struct{})
	page := 1

	for len(jobs) < wanted {
		select {
		case <-ctx.Done():
			return jobs, ctx.Err()
		default:
		}

		body, err := s.fetchPage(ctx, authToken, input, page)
		if err != nil {
			return nil, fmt.Errorf("infojobs page %d: %w", page, err)
		}

		var resp apiResponse
		if err := json.Unmarshal(body, &resp); err != nil {
			return nil, fmt.Errorf("infojobs decode: %w", err)
		}

		if len(resp.Items) == 0 {
			break
		}

		for _, raw := range resp.Items {
			if len(jobs) >= wanted {
				break
			}
			jobID := "infojobs-" + raw.ID
			if _, ok := seenIDs[jobID]; ok {
				continue
			}
			seenIDs[jobID] = struct{}{}

			job := mapJob(raw, input.DescriptionFormat)
			if job != nil {
				jobs = append(jobs, *job)
			}
		}

		if page >= resp.TotalPages {
			break
		}
		page++

		// Rate limit: 3 req/s → ~350ms between pages
		select {
		case <-ctx.Done():
			return jobs, ctx.Err()
		case <-time.After(350 * time.Millisecond):
		}
	}

	if !util.HasMeaningfulJobs(jobs) {
		return nil, fmt.Errorf("infojobs: no parseable jobs")
	}
	if len(jobs) > wanted {
		jobs = jobs[:wanted]
	}
	return jobs, nil
}

// fetchPage makes a single API request to InfoJobs.
func (s *Scraper) fetchPage(ctx context.Context, authToken string, input model.ScraperInput, page int) ([]byte, error) {
	urlStr := s.apiURL + "?page=" + fmt.Sprintf("%d", page)

	if v := strings.TrimSpace(input.SearchTerm); v != "" {
		urlStr += "&q=" + urlQueryEscape(v)
	}
	if v := strings.TrimSpace(input.Location); v != "" {
		urlStr += "&province=" + urlQueryEscape(v)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Basic "+authToken)
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; scrappy/1.0)")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}

	body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("read: %w", err)
	}
	return body, nil
}

// urlQueryEscape percent-encodes a query parameter value.
func urlQueryEscape(v string) string {
	escaped := strings.ReplaceAll(v, " ", "+")
	// Basic encoding for API compatibility
	return strings.ReplaceAll(escaped, "\"", "%22")
}

// mapJob converts a raw API offer into a JobPost.
func mapJob(raw apiOffer, descFormat string) *model.JobPost {
	if raw.ID == "" || raw.Title == "" {
		return nil
	}

	description := ""
	if raw.Description != nil {
		description = *raw.Description
		if descFormat == "plain" || descFormat == "" {
			description = stripHTMLTags(description)
		}
		// For "html" or "markdown", pass through as-is
	}

	// Build location from province and city
	loc := model.Location{}
	if raw.Province != nil {
		loc.State = raw.Province.Value
	}
	if raw.City != "" {
		loc.City = raw.City
	}

	// Determine if remote based on telework field
	isRemote := false
	if raw.TelpiOfferType != nil {
		t := strings.ToLower(*raw.TelpiOfferType)
		isRemote = strings.Contains(t, "remote") || strings.Contains(t, "teletrabajo") || strings.Contains(t, "telework")
	}

	// Parse date posted
	var datePosted *time.Time
	if raw.Published != "" {
		datePosted = util.ParseDatePosted(raw.Published)
		if datePosted == nil {
			// Try ISO format
			if t, err := time.Parse(time.RFC3339, raw.Published); err == nil {
				datePosted = &t
			}
		}
	}

	// Build job URL
	jobURL := raw.Link
	if jobURL == "" {
		jobURL = "https://www.infojobs.net/oferta/" + raw.ID
	}

	// Extract emails from description
	emails := extractEmails(description)

	return &model.JobPost{
		ID:          "infojobs-" + raw.ID,
		Title:       raw.Title,
		CompanyName: safeCompanyName(raw.Company),
		JobURL:      jobURL,
		Location:    loc,
		Description: description,
		IsRemote:    isRemote,
		DatePosted:  datePosted,
		Emails:      emails,
	}
}

// safeCompanyName extracts the company name from the API company object.
func safeCompanyName(c *apiCompany) string {
	if c == nil || strings.TrimSpace(c.Name) == "" {
		return ""
	}
	return strings.TrimSpace(c.Name)
}

// extractEmails finds email addresses in text and returns them as model.Email entries.
func extractEmails(text string) []model.Email {
	if text == "" {
		return nil
	}
	found := internalemail.Extract(text)
	if len(found) == 0 {
		return nil
	}
	out := make([]model.Email, 0, len(found))
	for _, e := range found {
		out = append(out, model.Email{
			Addr:   e.Addr,
			Role:   e.Role,
			Source: e.Source,
		})
	}
	return out
}

// stripHTMLTags removes HTML tags from a string, leaving plain text.
func stripHTMLTags(s string) string {
	var buf strings.Builder
	inTag := false
	for _, r := range s {
		if r == '<' {
			inTag = true
			continue
		}
		if r == '>' {
			inTag = false
			buf.WriteRune(' ')
			continue
		}
		if !inTag {
			buf.WriteRune(r)
		}
	}
	// Collapse whitespace
	result := strings.Join(strings.Fields(buf.String()), " ")
	return result
}

// ─── API types ─────────────────────────────────────────────────────────────────

type apiResponse struct {
	CurrentPage    int        `json:"currentPage"`
	PageSize       int        `json:"pageSize"`
	TotalPages     int        `json:"totalPages"`
	TotalResults   int        `json:"totalResults"`
	CurrentResults int        `json:"currentResults"`
	Items          []apiOffer `json:"items"`
}

type apiOffer struct {
	ID                string      `json:"id"`
	Title             string      `json:"title"`
	Province          *apiValue   `json:"province"`
	City              string      `json:"city"`
	Company           *apiCompany `json:"company"`
	Link              string      `json:"link"`
	SalaryMin         *apiValue   `json:"salaryMin"`
	SalaryMax         *apiValue   `json:"salaryMax"`
	SalaryDescription *string     `json:"salaryDescription"`
	ExperienceMin     *apiValue   `json:"experienceMin"`
	TelpiCallidadOffer bool       `json:"telpiCallidadOffer"`
	Description       *string     `json:"description"`
	MultiProvince     bool        `json:"multiProvince"`
	Published         string      `json:"published"`
	Updated           string      `json:"updated"`
	RequirementMin    *string     `json:"requirementMin"`
	TelpiOfferType    *string     `json:"telpiOfferType"`
}

type apiValue struct {
	ID    int    `json:"id"`
	Value string `json:"value"`
}

type apiCompany struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	URL  string `json:"url"`
}
