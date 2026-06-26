package joincom

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/scraper/ats"
	"github.com/arinbalyan/scrappy/internal/util"
)

const (
	defaultBaseURL      = "https://join.com"
	defaultAPIBaseURL   = "https://join.com/api/public"
	pageSize            = 50
	maxPages            = 100
	defaultLocale       = "en-us"
	defaultResultsWanted = 100
)

// Regex patterns for company ID extraction from HTML.
var (
	primaryIDRegex   = regexp.MustCompile(`"company":\{"id":(\d+)`)
	fallbackIDRegex  = regexp.MustCompile(`"companyId":(\d+)`)
)

// Location represents a location from Join.com.
type Location struct {
	ID      interface{} `json:"id"`
	Name    *string     `json:"name"`
	City    *string     `json:"city"`
	Country *string     `json:"country"`
	IsRemote *bool      `json:"isRemote"`
}

// Pagination represents pagination info.
type Pagination struct {
	Page       *int `json:"page"`
	PageSize   *int `json:"pageSize"`
	Total      *int `json:"total"`
	TotalPages *int `json:"totalPages"`
}

// DepartmentShim handles the fact that department can be a string or object.
type DepartmentShim struct {
	Name *string `json:"name"`
}

// JobItem represents a single job from Join.com.
type JobItem struct {
	ID             interface{}   `json:"id"`
	Title          *string       `json:"title"`
	Description    *string       `json:"description"`
	Locations      []Location    `json:"locations"`
	ShareableURL   *string       `json:"shareableUrl"`
	PublishedAt    *string       `json:"publishedAt"`
	EmploymentType *string       `json:"employmentType"`
	RemoteOption   *string       `json:"remoteOption"`
	Category       *struct {
		Name *string `json:"name"`
	} `json:"category"`
	Department     *DepartmentShim `json:"department"`
}

// JobsPage represents the paginated response.
type JobsPage struct {
	Items      []JobItem   `json:"items"`
	Pagination *Pagination `json:"pagination"`
}

// Scraper for Join.com.
type Scraper struct {
	Client   *http.Client
	baseURL  string
	apiURL   string
}

// New creates a new Join.com scraper.
func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Retries: 2, Timeout: 20 * time.Second})
	}
	return &Scraper{Client: client, baseURL: defaultBaseURL, apiURL: defaultAPIBaseURL}
}

// NewWithAPIURL creates a scraper with a custom base URL. The apiURL is treated as the base URL
// when provided (for test compatibility).
func NewWithAPIURL(client *http.Client, apiURL string) *Scraper {
	s := New(client)
	if apiURL != "" {
		s.apiURL = apiURL
		s.baseURL = apiURL
	}
	return s
}

// SiteName returns the site identifier.
func (s *Scraper) SiteName() model.Site { return model.SiteJoinCom }

// Scrape fetches jobs from Join.com.
func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{"site": s.SiteName(), "results_wanted": input.ResultsWanted, "search_term": input.SearchTerm})

	seeds, src := ats.ResolveSeeds(input.SearchTerm, "SCRAPPY_JOINCOM_SEEDS")
	if len(seeds) == 0 {
		return nil, fmt.Errorf("joincom no seeds: set SCRAPPY_JOINCOM_SEEDS or pass a company name in --search")
	}
	util.Debug("joincom_seeds", map[string]any{"seeds": seeds, "src": src})

	wanted := input.ResultsWanted
	if wanted <= 0 {
		wanted = defaultResultsWanted
	}

	seen := map[string]struct{}{}
	var mu sync.Mutex

	fetchFn := func(ctx context.Context, slug string) ([]model.JobPost, error) {
		jobs, err := s.collectJobs(ctx, input, slug)
		if err != nil {
			util.Warn("joincom_seed_fail", map[string]any{"seed": slug, "err": err.Error()})
			return nil, err
		}
		var result []model.JobPost
		for _, jp := range jobs {
			mu.Lock()
			if _, ok := seen[jp.ID]; ok {
				mu.Unlock()
				continue
			}
			seen[jp.ID] = struct{}{}
			mu.Unlock()
			result = append(result, jp)
		}
		return result, nil
	}

	results := ats.ProcessSeeds(ctx, seeds, 3, wanted, fetchFn)
	if !util.HasMeaningfulJobs(results) {
		return nil, fmt.Errorf("joincom no parseable jobs")
	}
	return results, nil
}

type tenantContext struct {
	companyID   int
	companySlug string
	companyName string
}

func (s *Scraper) collectJobs(ctx context.Context, input model.ScraperInput, slug string) ([]model.JobPost, error) {
	// Step 1: resolve tenant by scraping the company HTML page for the company ID
	tenant, err := s.resolveTenant(ctx, slug)
	if err != nil {
		return nil, fmt.Errorf("joincom resolve tenant %s: %w", slug, err)
	}

	resultsWanted := input.ResultsWanted
	if resultsWanted <= 0 {
		resultsWanted = defaultResultsWanted
	}

	// Step 2: paginated API calls to collect job items
	var out []model.JobPost
	currentPage := 1
	totalPages := 1

	for currentPage <= maxPages && len(out) < resultsWanted {
		page, err := s.fetchPage(ctx, tenant.companyID, currentPage)
		if err != nil {
			return out, fmt.Errorf("joincom page %d: %w", currentPage, err)
		}

		if len(page.Items) == 0 {
			break
		}

		for _, item := range page.Items {
			if len(out) >= resultsWanted {
				break
			}
			jp := s.mapJob(item, tenant)
			if jp != nil {
				out = append(out, *jp)
			}
		}

		if page.Pagination != nil && page.Pagination.TotalPages != nil {
			totalPages = *page.Pagination.TotalPages
		}
		if currentPage >= totalPages {
			break
		}
		currentPage++

		// Polite pacing (0.5s between pages)
		select {
		case <-ctx.Done():
			return out, ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}

	return out, nil
}

func (s *Scraper) resolveTenant(ctx context.Context, slug string) (*tenantContext, error) {
	// Fetch HTML page
	htmlURL := fmt.Sprintf("%s/companies/%s", s.baseURL, slug)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, htmlURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := s.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch html: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}

	body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("read: %w", err)
	}
	html := string(body)

	// Extract company ID with regex
	var idStr string
	matches := primaryIDRegex.FindStringSubmatch(html)
	if len(matches) >= 2 {
		idStr = matches[1]
	} else {
		matches = fallbackIDRegex.FindStringSubmatch(html)
		if len(matches) >= 2 {
			idStr = matches[1]
		}
	}

	if idStr == "" {
		return nil, fmt.Errorf("no company id found in HTML")
	}

	companyID, err := strconv.Atoi(idStr)
	if err != nil || companyID <= 0 {
		return nil, fmt.Errorf("invalid company id: %s", idStr)
	}

	return &tenantContext{
		companyID:   companyID,
		companySlug: slug,
		companyName: deriveCompanyName(slug),
	}, nil
}

func (s *Scraper) fetchPage(ctx context.Context, companyID int, page int) (*JobsPage, error) {
	url := fmt.Sprintf("%s/companies/%d/jobs?locale=%s&page=%d&pageSize=%d&withAggregations=true&sort=+title",
		s.apiURL, companyID, defaultLocale, page, pageSize)

	var result JobsPage
	if err := ats.FetchJSON(ctx, s.Client, url, &result); err != nil {
		return nil, err
	}
	if result.Items == nil {
		result.Items = []JobItem{}
	}
	return &result, nil
}

func (s *Scraper) mapJob(item JobItem, tenant *tenantContext) *model.JobPost {
	id := fmt.Sprintf("%v", item.ID)
	if id == "" || id == "<nil>" {
		return nil
	}

	title := ""
	if item.Title != nil {
		title = *item.Title
	}
	if title == "" {
		return nil
	}

	// Location
	location := model.Location{}
	isRemote := false
	if len(item.Locations) > 0 {
		loc := item.Locations[0]
		if loc.Name != nil && *loc.Name != "" {
			location.City = *loc.Name
		} else if loc.City != nil && *loc.City != "" {
			location.City = *loc.City
		}
		if loc.Country != nil {
			location.Country = *loc.Country
		}
		if loc.IsRemote != nil {
			isRemote = *loc.IsRemote
		}
	}

	// Also check remoteOption
	if !isRemote && item.RemoteOption != nil {
		isRemote = strings.Contains(strings.ToLower(*item.RemoteOption), "remote")
	}

	// Check location name for "remote"
	if !isRemote && len(item.Locations) > 0 {
		if item.Locations[0].Name != nil {
			isRemote = strings.Contains(strings.ToLower(*item.Locations[0].Name), "remote")
		}
	}

	// Description
	description := ""
	if item.Description != nil {
		description = *item.Description
	}

	// Department
	department := ""
	if item.Department != nil && item.Department.Name != nil {
		department = *item.Department.Name
	}
	if department == "" && item.Category != nil && item.Category.Name != nil {
		department = *item.Category.Name
	}

	// Job URL
	jobURL := ""
	if item.ShareableURL != nil && *item.ShareableURL != "" {
		jobURL = *item.ShareableURL
	}
	if jobURL == "" {
		jobURL = fmt.Sprintf("%s/jobs/%s", s.baseURL, id)
	}

	// Date posted
	var datePosted *time.Time
	if item.PublishedAt != nil && *item.PublishedAt != "" {
		datePosted = util.ParseDatePosted(*item.PublishedAt)
	}

	// Employment type
	employmentType := ""
	if item.EmploymentType != nil {
		employmentType = *item.EmploymentType
	}

	return &model.JobPost{
		ID:            ats.BuildID("joincom", tenant.companySlug, id),
		Title:         title,
		CompanyName:   tenant.companyName,
		JobURL:        jobURL,
		Location:      location,
		IsRemote:      isRemote,
		Description:   description,
		DatePosted:    datePosted,
		Site:          string(s.SiteName()),
		Department:    department,
		JobType:       employmentType,
	}
}

func deriveCompanyName(slug string) string {
	return strings.Title(strings.ReplaceAll(slug, "-", " "))
}
