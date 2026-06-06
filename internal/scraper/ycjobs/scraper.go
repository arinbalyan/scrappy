package ycjobs

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/util"
)

// ─── Algolia constants ─────────────────────────────────────────────────────────
const (
	algoliaAppID               = "45BWZJ1SGC"
	algoliaQueryURL            = "https://45bwzj1sgc-dsn.algolia.net/1/indexes/*/queries"
	jobsIndexCreatedAtDesc     = "WaaSPublicCompanyJob_created_at_desc_production"
	jobsIndexRelevance         = "WaaSPublicCompanyJob_production"
	defaultHitsPerPage         = 100
	workatastartupPage         = "https://www.workatastartup.com"
)

// ─── Regex patterns ────────────────────────────────────────────────────────────

// reAPIKey matches the base64 Algolia API key embedded in the JS bundle.
// It appears as: x-algolia-api-key:"<base64>"
var reAPIKey = regexp.MustCompile(`x-algolia-api-key[:\s]*"([^"]+)"`)

// ─── Algolia request/response types ────────────────────────────────────────────

type algoliaRequest struct {
	Requests []algoliaSubRequest `json:"requests"`
}

type algoliaSubRequest struct {
	IndexName string `json:"indexName"`
	Params    string `json:"params"`
}

type algoliaResponse struct {
	Results []algoliaIndexResult `json:"results"`
}

type algoliaIndexResult struct {
	Hits     []algoliaHit `json:"hits"`
	Page     int          `json:"page"`
	NbPages  int          `json:"nbPages"`
	NbHits   int          `json:"nbHits"`
}

type algoliaHit struct {
	ID                int      `json:"id"`
	ObjectID          string   `json:"objectID"`
	Title             string   `json:"title"`
	Description       string   `json:"description,omitempty"`
	CompanyID         int      `json:"company_id"`
	CompanyName       string   `json:"company_name"`
	CompanyWebsite    string   `json:"company_website,omitempty"`
	Remote            string   `json:"remote,omitempty"` // "only" | "yes" | "no"
	Role              string   `json:"role,omitempty"`
	EngType           []string `json:"eng_type,omitempty"`
	LocationsForSearch []string `json:"locations_for_search,omitempty"`
	JobType           string   `json:"job_type,omitempty"`
	MinExperience     int      `json:"min_experience,omitempty"`
	HasSalary         bool     `json:"has_salary"`
	HasEquity         bool     `json:"has_equity"`
	Skills            []string `json:"skills,omitempty"`
	SalaryRange       string   `json:"salary_range,omitempty"`
	EquityRange       string   `json:"equity_range,omitempty"`
	CreatedAt         string   `json:"created_at,omitempty"`
	SearchPath        string   `json:"search_path,omitempty"`
	CompanyWaaSStage  string   `json:"company_waas_stage,omitempty"`
	CompanyParentSector string `json:"company_parent_sector,omitempty"`
	USVisaRequired   string   `json:"us_visa_required,omitempty"`
}

// ─── Scraper ───────────────────────────────────────────────────────────────────

type Scraper struct {
	client     *http.Client
	listURL    string // when set, bypass Algolia and use HTML scraping (test mode)
	apiKey     string
	apiKeyOnce sync.Once
	apiKeyErr  error
}

func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Retries: 2, Timeout: 30 * time.Second})
	}
	return &Scraper{client: client}
}

// NewWithListURL is used by contract tests to inject a mock server URL.
// When listURL is set, the scraper falls back to HTML-based scraping at that URL
// (bypassing Algolia API discovery).
func NewWithListURL(client *http.Client, listURL string) *Scraper {
	s := New(client)
	if strings.TrimSpace(listURL) != "" {
		s.listURL = strings.TrimSpace(listURL)
	}
	return s
}

func (s *Scraper) SiteName() model.Site { return model.SiteYCJobs }

func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	// When listURL is set (test mode), bypass Algolia and use HTML scraping at that URL.
	if s.listURL != "" {
		return s.scrapeHTMLAtURL(ctx, input, s.listURL)
	}

	apiKey, err := s.ensureAPIKey(ctx)
	if err != nil {
		util.Warn("ycjobs_api_key_discovery_failed", map[string]any{"err": err.Error()})
		// Fall back to server-rendered HTML.
		return s.scrapeHTMLFallback(ctx, input)
	}

	var allJobs []model.JobPost
	page := 0

	for {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("ycjobs: %w", ctx.Err())
		}

		resp, err := s.queryAlgolia(ctx, apiKey, input, page)
		if err != nil {
			return nil, fmt.Errorf("ycjobs page %d: %w", page, err)
		}

		for _, hit := range resp.Hits {
			job := hitToJobPost(hit)
			if job.Title == "" {
				continue
			}
			allJobs = append(allJobs, job)
		}

		if input.ResultsWanted > 0 && len(allJobs) >= input.ResultsWanted {
			allJobs = allJobs[:input.ResultsWanted]
			break
		}

		if page+1 >= resp.NbPages {
			break
		}
		page++
	}

	if !util.HasMeaningfulJobs(allJobs) {
		return nil, fmt.Errorf("ycjobs no parseable jobs")
	}
	util.Debug("ycjobs_result", map[string]any{"count": len(allJobs), "pages": page + 1})
	return allJobs, nil
}

// ─── Algolia API Key Discovery ─────────────────────────────────────────────────

func (s *Scraper) ensureAPIKey(ctx context.Context) (string, error) {
	s.apiKeyOnce.Do(func() {
		s.apiKey, s.apiKeyErr = s.fetchAlgoliaAPIKey(ctx)
	})
	return s.apiKey, s.apiKeyErr
}

func (s *Scraper) fetchAlgoliaAPIKey(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, workatastartupPage, nil)
	if err != nil {
		return "", fmt.Errorf("request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Accept", "text/html")

	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("status %d", resp.StatusCode)
	}

	body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return "", fmt.Errorf("read: %w", err)
	}

	// Search for the Algolia API key in the HTML.
	// The key appears in inline scripts or the page data.
	bodyStr := string(body)

	// First try direct pattern in the HTML (inline config).
	if m := reAPIKey.FindStringSubmatch(bodyStr); len(m) >= 2 {
		key := strings.TrimSpace(m[1])
		if key != "" {
			util.Debug("ycjobs_api_key_found_inline", nil)
			return key, nil
		}
	}

	// If not found inline, look for JS chunk URLs and search there.
	jsURLs := extractJSChunkURLs(bodyStr)
	for _, jsURL := range jsURLs {
		absURL := resolveURL(workatastartupPage, jsURL)
		key, err := s.searchJSForKey(ctx, absURL)
		if err == nil && key != "" {
			util.Debug("ycjobs_api_key_found_in_js", map[string]any{"url": absURL})
			return key, nil
		}
	}

	return "", fmt.Errorf("no algolia api key found in page")
}

// extractJSChunkURLs finds JavaScript bundle URLs in the HTML.
func extractJSChunkURLs(html string) []string {
	re := regexp.MustCompile(`src=["']([^"']+\.js[^"']*)["']`)
	matches := re.FindAllStringSubmatch(html, -1)
	var urls []string
	seen := map[string]bool{}
	for _, m := range matches {
		u := strings.TrimSpace(m[1])
		if u != "" && !seen[u] {
			seen[u] = true
			urls = append(urls, u)
		}
	}
	return urls
}

func (s *Scraper) searchJSForKey(ctx context.Context, jsURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, jsURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return "", err
	}

	if m := reAPIKey.FindStringSubmatch(string(body)); len(m) >= 2 {
		return strings.TrimSpace(m[1]), nil
	}
	return "", fmt.Errorf("key not found in %s", jsURL)
}

// resolveURL resolves a relative URL against a base.
func resolveURL(base, ref string) string {
	if strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://") {
		return ref
	}
	baseURL, err := url.Parse(base)
	if err != nil {
		return ref
	}
	refURL, err := url.Parse(ref)
	if err != nil {
		return ref
	}
	return baseURL.ResolveReference(refURL).String()
}

// ─── Algolia Query ─────────────────────────────────────────────────────────────

func (s *Scraper) queryAlgolia(ctx context.Context, apiKey string, input model.ScraperInput, page int) (*algoliaIndexResult, error) {
	// Build Algolia params.
	params := url.Values{}
	params.Set("query", input.SearchTerm)
	params.Set("page", fmt.Sprintf("%d", page))
	params.Set("hitsPerPage", fmt.Sprintf("%d", defaultHitsPerPage))
	params.Set("attributesToRetrieve", `["*"]`)
	params.Set("attributesToHighlight", "[]")
	params.Set("attributesToSnippet", "[]")
	params.Set("clickAnalytics", "true")
	params.Set("distinct", "true")

	// Add filters if specified.
	var filters []string
	if input.RemoteOnly {
		filters = append(filters, `remote:"only"`)
	}
	if input.IsRemote {
		filters = append(filters, `(remote:"only" OR remote:"yes")`)
	}

	// Convert search terms to role-based filters when possible.
	if input.SearchTerm != "" {
		roleFilter := searchTermToRoleFilter(input.SearchTerm)
		if roleFilter != "" {
			filters = append(filters, roleFilter)
		}
	}

	if len(filters) > 0 {
		params.Set("filters", strings.Join(filters, " AND "))
	}

	// Choose index based on whether there's a search term.
	indexName := jobsIndexCreatedAtDesc
	if input.SearchTerm != "" {
		indexName = jobsIndexRelevance
	}

	reqBody := algoliaRequest{
		Requests: []algoliaSubRequest{
			{
				IndexName: indexName,
				Params:    params.Encode(),
			},
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, algoliaQueryURL, strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("x-algolia-application-id", algoliaAppID)
	req.Header.Set("x-algolia-api-key", apiKey)
	req.Header.Set("Origin", "https://www.workatastartup.com")
	req.Header.Set("Referer", "https://www.workatastartup.com/")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("algolia request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := util.ReadBodyLimited(resp.Body, 4096)
		return nil, fmt.Errorf("algolia status %d: %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
	}

	var algoliaResp algoliaResponse
	if err := json.NewDecoder(resp.Body).Decode(&algoliaResp); err != nil {
		return nil, fmt.Errorf("algolia decode: %w", err)
	}

	if len(algoliaResp.Results) == 0 {
		return &algoliaIndexResult{}, nil
	}

	return &algoliaResp.Results[0], nil
}

// ─── Hit Conversion ────────────────────────────────────────────────────────────

func hitToJobPost(hit algoliaHit) model.JobPost {
	// Build the canonical Y Combinator job URL.
	jobURL := hit.SearchPath
	if jobURL == "" {
		jobURL = fmt.Sprintf("https://www.ycombinator.com/companies/%s/jobs/%d", urlize(hit.CompanyName), hit.ID)
	}

	// Determine remote status.
	remote := hit.Remote == "only" || hit.Remote == "yes"

	// Determine job type.
	jobType := hit.JobType
	if jobType == "" {
		jobType = "fulltime"
	}

	// Determine salary range.
	var compensation model.Compensation
	if hit.SalaryRange != "" {
		compensation.Currency = "USD"
		// Parse "$200K - $250K" or "$100K" format.
		compensation = parseSalaryRange(hit.SalaryRange)
	}

	// Parse created_at.
	var postedAt *time.Time
	if hit.CreatedAt != "" {
		if t, err := time.Parse(time.RFC3339, hit.CreatedAt); err == nil {
			postedAt = &t
		}
	}

	// Build skills string for description enrichment.
	desc := hit.Description
	if len(hit.Skills) > 0 {
		if desc != "" {
			desc += "\n\nSkills: " + strings.Join(hit.Skills, ", ")
		}
	}

	return model.JobPost{
		ID:              fmt.Sprintf("ycjobs-%d", hit.ID),
		Title:           hit.Title,
		CompanyName:     hit.CompanyName,
		CompanyURL:      hit.CompanyWebsite,
		JobURL:          jobURL,
		IsRemote:        remote,
		Description:     desc,
		Compensation:    &compensation,
		JobType:         jobType,
		DatePosted:      postedAt,
		Skills:          hit.Skills,
	}
}

// searchTermToRoleFilter attempts to convert a free-text search term to
// an Algolia role filter. Returns "" if no role match is found.
func searchTermToRoleFilter(search string) string {
	lower := strings.ToLower(search)
	var roles []string

	roleMap := map[string][]string{
		"engineer":    {"eng"},
		"engineering": {"eng"},
		"eng":         {"eng"},
		"backend":     {"eng"},
		"back end":    {"eng"},
		"frontend":    {"eng"},
		"front end":   {"eng"},
		"fullstack":   {"eng"},
		"full stack":  {"eng"},
		"ml":          {"eng"},
		"machine learning": {"eng"},
		"ai":          {"eng"},
		"data":        {"science", "eng"},
		"data science": {"science"},
		"scientist":   {"science"},
		"designer":    {"design"},
		"design":      {"design"},
		"product":     {"product"},
		"pm":          {"product"},
		"sales":       {"sales"},
		"marketing":   {"marketing"},
		"operations":  {"operations"},
		"hr":          {"people"},
		"recruiter":   {"people"},
	}

	for word, mappedRoles := range roleMap {
		if strings.Contains(lower, word) {
			for _, r := range mappedRoles {
				roles = append(roles, fmt.Sprintf(`role:"%s"`, r))
			}
		}
	}

	if len(roles) == 0 {
		return ""
	}

	// Deduplicate.
	seen := map[string]bool{}
	var unique []string
	for _, r := range roles {
		if !seen[r] {
			seen[r] = true
			unique = append(unique, r)
		}
	}

	if len(unique) == 1 {
		return unique[0]
	}
	return "(" + strings.Join(unique, " OR ") + ")"
}

// ─── Parsing helpers ───────────────────────────────────────────────────────────

// parseSalaryRange converts "$200K - $250K" or "$100K" to Compensation.
func parseSalaryRange(s string) model.Compensation {
	var c model.Compensation
	c.Currency = "USD"
	c.Interval = model.IntervalYearly

	// Remove commas, "$", etc.
	cleaned := strings.NewReplacer("$", "", ",", "", " ", "").Replace(s)

	// Try "200K-250K" or "200K–250K" formats.
	parts := regexp.MustCompile(`[-–—]+`).Split(cleaned, 2)
	if len(parts) == 0 {
		return c
	}
	minVal := parseKString(parts[0])
	if minVal > 0 {
		c.MinAmount = &minVal
	}
	if len(parts) >= 2 {
		maxVal := parseKString(parts[1])
		if maxVal > 0 {
			c.MaxAmount = &maxVal
		}
	}
	return c
}

func parseKString(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	multiplier := 1.0
	if strings.HasSuffix(s, "K") || strings.HasSuffix(s, "k") {
		multiplier = 1000.0
		s = strings.TrimSuffix(strings.TrimSuffix(s, "K"), "k")
	}
	if strings.HasSuffix(s, "M") || strings.HasSuffix(s, "m") {
		multiplier = 1000000.0
		s = strings.TrimSuffix(strings.TrimSuffix(s, "M"), "m")
	}
	val, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0
	}
	return val * multiplier
}

// urlize converts a company name to a URL-friendly slug.
func urlize(name string) string {
	slug := strings.ToLower(name)
	slug = strings.NewReplacer(
		" ", "-",
		".", "-",
		",", "",
		"'", "",
		"\"", "",
		"(", "",
		")", "",
		"&", "and",
	).Replace(slug)
	// Collapse multiple dashes.
	re := regexp.MustCompile(`-+`)
	slug = re.ReplaceAllString(slug, "-")
	return strings.Trim(slug, "-")
}

// ─── HTML Fallback ────────────────────────────────────────────────────────────

// scrapeHTMLFallback uses regex-based HTML scraping as a fallback
// when the Algolia API key cannot be discovered.
var reJob = regexp.MustCompile(`(?is)<a[^>]*href="([^"]+)"[^>]*>([^<]{4,140})</a>`)

func (s *Scraper) scrapeHTMLFallback(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	return s.scrapeHTMLAtURL(ctx, input, "https://www.ycombinator.com/jobs")
}

func (s *Scraper) scrapeHTMLAtURL(ctx context.Context, input model.ScraperInput, pageURL string) ([]model.JobPost, error) {
	util.Debug("ycjobs_html_scrape", map[string]any{"url": pageURL})

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ycjobs html request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("ycjobs html status %d", resp.StatusCode)
	}
	b, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("ycjobs html read: %w", err)
	}
	m := reJob.FindAllStringSubmatch(string(b), -1)
	out := make([]model.JobPost, 0, len(m))
	seen := map[string]struct{}{}
	for i, row := range m {
		u := strings.TrimSpace(row[1])
		if strings.HasPrefix(u, "/") {
			u = pageURL + u
		}
		if !strings.HasPrefix(u, "http") {
			continue
		}
		if _, ok := seen[u]; ok {
			continue
		}
		seen[u] = struct{}{}
		title := strings.TrimSpace(row[2])
		if title == "" {
			continue
		}
		out = append(out, model.JobPost{
			ID:       fmt.Sprintf("ycjobs-html-%d", i+1),
			Title:    title,
			JobURL:   u,
			IsRemote: true,
		})
		if input.ResultsWanted > 0 && len(out) >= input.ResultsWanted {
			break
		}
	}
	if !util.HasMeaningfulJobs(out) {
		return nil, fmt.Errorf("ycjobs no parseable jobs")
	}
	return out, nil
}
