package tesla

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
	baseURL          = "https://www.tesla.com"
	boardPath        = "/cua-api/apps/careers/state"
	detailPathTmpl   = "/cua-api/careers/job/%s"
	publicJobBase    = "https://www.tesla.com/careers/search/job"
	defaultWanted    = 100
	detailBudget     = 25 // fetch description for first N jobs
	maxBodyBytes     = 8 * 1024 * 1024
)

// boardResponse maps the Tesla board API response envelope.
type boardResponse struct {
	Listings []boardListing         `json:"listings"`
	Lookup   *boardLookup           `json:"lookup,omitempty"`
}

type boardListing struct {
	ID string `json:"id"`
	T  string `json:"t"`
	L  string `json:"l,omitempty"`
	D  string `json:"d,omitempty"`
	R  string `json:"r,omitempty"`
}

type boardLookup struct {
	Locations   map[string]string `json:"locations,omitempty"`
	Departments map[string]string `json:"departments,omitempty"`
	Regions     map[string]string `json:"regions,omitempty"`
}

// jobDetail maps the Tesla per-job detail API response.
type jobDetail struct {
	JobDescription               *string `json:"jobDescription,omitempty"`
	JobResponsibilities          *string `json:"jobResponsibilities,omitempty"`
	JobRequirements              *string `json:"jobRequirements,omitempty"`
	JobCompensationAndBenefits   *string `json:"jobCompensationAndBenefits,omitempty"`
	Department                   *string `json:"department,omitempty"`
	TimeType                     *string `json:"timeType,omitempty"`
}

// Scraper fetches jobs from the Tesla careers API.
type Scraper struct {
	client  *http.Client
	baseURL string
}

// New creates a new Tesla scraper.
func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{
			Retries: 2,
			Timeout: 20 * time.Second,
			UserAgents: []string{
				"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/143.0.0.0 Safari/537.36",
			},
		})
	}
	return &Scraper{client: client, baseURL: baseURL}
}

// NewWithBaseURL creates a new scraper with a custom base URL (used in tests).
func NewWithBaseURL(client *http.Client, baseURL string) *Scraper {
	s := New(client)
	if strings.TrimSpace(baseURL) != "" {
		s.baseURL = strings.TrimSpace(baseURL)
	}
	return s
}

// SiteName returns the site identifier.
func (s *Scraper) SiteName() model.Site { return model.SiteTesla }

// Scrape fetches jobs from the Tesla careers API.
func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{
		"site":           s.SiteName(),
		"results_wanted": input.ResultsWanted,
	})

	wanted := input.ResultsWanted
	if wanted <= 0 {
		wanted = defaultWanted
	}

	board, err := s.fetchBoard(ctx)
	if err != nil {
		return nil, fmt.Errorf("tesla: board fetch: %w", err)
	}
	if board == nil || len(board.Listings) == 0 {
		return nil, fmt.Errorf("tesla: no jobs in board response")
	}

	if wanted > len(board.Listings) {
		wanted = len(board.Listings)
	}
	listings := board.Listings[:wanted]

	// Determine how many detail descriptions to fetch
	budget := detailBudget
	if !isFinite(float64(budget)) {
		budget = len(listings)
	}

	jobs := make([]model.JobPost, 0, len(listings))
	for i, listing := range listings {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		var description string
		if i < budget {
			if d, err := s.fetchDetail(ctx, listing.ID); err == nil && d != "" {
				description = d
			}
		}

		job := s.toJobPost(listing, board.Lookup, description)
		jobs = append(jobs, job)
	}

	util.Debug("scraper_done", map[string]any{
		"site": s.SiteName(),
		"jobs": len(jobs),
	})

	if !util.HasMeaningfulJobs(jobs) {
		return nil, fmt.Errorf("tesla: no parseable jobs")
	}
	return jobs, nil
}

// fetchBoard retrieves the full job catalogue from the Tesla board API.
func (s *Scraper) fetchBoard(ctx context.Context) (*boardResponse, error) {
	url := s.baseURL + boardPath

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/143.0.0.0 Safari/537.36")
	req.Header.Set("Referer", s.baseURL+"/careers/search/")
	req.Header.Set("Origin", s.baseURL)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusServiceUnavailable {
		return nil, fmt.Errorf("akamai challenge (HTTP %d) — try using --proxy with a residential proxy", resp.StatusCode)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := util.ReadBodyLimited(resp.Body, maxBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	// Check for Akamai HTML challenge
	if looksLikeHTML(body) {
		return nil, fmt.Errorf("akamai challenge (HTML body)")
	}

	var board boardResponse
	if err := json.Unmarshal(body, &board); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	return &board, nil
}

// fetchDetail retrieves the full job description for a single listing.
func (s *Scraper) fetchDetail(ctx context.Context, jobID string) (string, error) {
	url := s.baseURL + fmt.Sprintf(detailPathTmpl, jobID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Referer", s.baseURL+"/careers/search/")
	req.Header.Set("Origin", s.baseURL)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/143.0.0.0 Safari/537.36")

	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := util.ReadBodyLimited(resp.Body, maxBodyBytes)
	if err != nil {
		return "", err
	}

	var detail jobDetail
	if err := json.Unmarshal(body, &detail); err != nil {
		return "", err
	}

	return composeDescription(&detail), nil
}

// composeDescription concatenates the four detail fields with \n\n separators.
func composeDescription(detail *jobDetail) string {
	if detail == nil {
		return ""
	}
	parts := make([]string, 0, 4)
	if detail.JobDescription != nil && strings.TrimSpace(*detail.JobDescription) != "" {
		parts = append(parts, "Description:\n"+strings.TrimSpace(*detail.JobDescription))
	}
	if detail.JobResponsibilities != nil && strings.TrimSpace(*detail.JobResponsibilities) != "" {
		parts = append(parts, "Responsibilities:\n"+strings.TrimSpace(*detail.JobResponsibilities))
	}
	if detail.JobRequirements != nil && strings.TrimSpace(*detail.JobRequirements) != "" {
		parts = append(parts, "Requirements:\n"+strings.TrimSpace(*detail.JobRequirements))
	}
	if detail.JobCompensationAndBenefits != nil && strings.TrimSpace(*detail.JobCompensationAndBenefits) != "" {
		parts = append(parts, "Compensation & Benefits:\n"+strings.TrimSpace(*detail.JobCompensationAndBenefits))
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "\n\n")
}

// toJobPost maps a board listing + lookup + description to a JobPost.
func (s *Scraper) toJobPost(listing boardListing, lookup *boardLookup, description string) model.JobPost {
	job := model.JobPost{
		ID:          "tesla-" + listing.ID,
		Title:       listing.T,
		CompanyName: "Tesla",
		JobURL:      buildJobURL(listing.ID, listing.T),
		Description: description,
		Site:        string(s.SiteName()),
		ApplyMethod: "external_url",
	}

	// Resolve location from lookup
	if lookup != nil && listing.L != "" {
		if loc, ok := lookup.Locations[listing.L]; ok && loc != "" {
			job.Location = model.Location{City: loc}
			job.IsRemote = strings.Contains(strings.ToLower(loc), "remote")
		}
	}

	// Resolve department from lookup
	if lookup != nil && listing.D != "" {
		if dept, ok := lookup.Departments[listing.D]; ok && dept != "" {
			job.Department = dept
		}
	}

	return job
}

// buildJobURL constructs the public Tesla career page URL.
func buildJobURL(jobID, title string) string {
	slug := slugify(title)
	return fmt.Sprintf("%s/%s-%s", publicJobBase, slug, jobID)
}

// slugify converts a title to a kebab-case slug.
func slugify(text string) string {
	lower := strings.ToLower(text)
	// Replace non-alphanumeric with hyphens
	result := make([]byte, 0, len(lower))
	for i := 0; i < len(lower); i++ {
		c := lower[i]
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') {
			result = append(result, c)
		} else {
			if len(result) > 0 && result[len(result)-1] != '-' {
				result = append(result, '-')
			}
		}
	}
	// Trim trailing hyphens
	for len(result) > 0 && result[len(result)-1] == '-' {
		result = result[:len(result)-1]
	}
	return string(result)
}

// looksLikeHTML checks if body bytes look like an HTML page rather than JSON.
func looksLikeHTML(body []byte) bool {
	s := string(body)
	s = strings.TrimSpace(s)
	if len(s) == 0 {
		return false
	}
	return strings.HasPrefix(s, "<") || strings.HasPrefix(s, "<!") ||
		strings.HasPrefix(s, "<html") || strings.HasPrefix(s, "<HTML")
}

// isFinite checks if a float64 is not NaN and not +Inf/-Inf.
func isFinite(f float64) bool {
	return !(f != f || f > 1e308 || f < -1e308)
}
