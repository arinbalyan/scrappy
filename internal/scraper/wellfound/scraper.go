package wellfound

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
	jobsURL = "https://wellfound.com/jobs"
)

// Scraper fetches jobs from Wellfound (AngelList) by scraping the
// __NEXT_DATA__ JSON blob embedded in the publicly rendered HTML.
type Scraper struct {
	client  *http.Client
	baseURL string
}

// New creates a new Wellfound scraper.
func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{
			Timeout:  30 * time.Second,
			Retries:  2,
			UserAgents: []string{
				"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/129.0.0.0 Safari/537.36",
			},
		})
	}
	return &Scraper{client: client, baseURL: jobsURL}
}

// NewWithBaseURL creates a scraper with a custom base URL (used in tests).
func NewWithBaseURL(client *http.Client, baseURL string) *Scraper {
	s := New(client)
	if strings.TrimSpace(baseURL) != "" {
		s.baseURL = strings.TrimSpace(baseURL)
	}
	return s
}

// SiteName returns the site identifier.
func (s *Scraper) SiteName() model.Site { return model.SiteWellfound }

// --- Response types ---

type nextData struct {
	Props *pagePropsWrapper `json:"props,omitempty"`
}

type pagePropsWrapper struct {
	PageProps *pageProps `json:"pageProps,omitempty"`
}

type pageProps struct {
	Jobs     []listing `json:"jobs,omitempty"`
	Listings []listing `json:"listings,omitempty"`
}

type listing struct {
	ID          json.Number   `json:"id"`
	Title       string        `json:"title"`
	Slug        string        `json:"slug,omitempty"`
	Company     *company      `json:"company,omitempty"`
	Compensation *compensation `json:"compensation,omitempty"`
	Locations   []string      `json:"locations,omitempty"`
	Remote      bool          `json:"remote,omitempty"`
	Description string        `json:"description,omitempty"`
	Skills      []string      `json:"skills,omitempty"`
	CreatedAt   string        `json:"createdAt,omitempty"`
}

type company struct {
	Name    string `json:"name"`
	Slug    string `json:"slug,omitempty"`
	LogoURL string `json:"logoUrl,omitempty"`
}

type compensation struct {
	Min      *float64 `json:"min,omitempty"`
	Max      *float64 `json:"max,omitempty"`
	Currency string   `json:"currency,omitempty"`
}

// Scrape fetches jobs from Wellfound.
func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{
		"site":           s.SiteName(),
		"results_wanted": input.ResultsWanted,
		"search_term":    input.SearchTerm,
	})

	wanted := input.ResultsWanted
	if wanted <= 0 {
		wanted = 15
	}

	url := s.baseURL
	if strings.TrimSpace(input.SearchTerm) != "" {
		url = url + "?q=" + strings.ReplaceAll(input.SearchTerm, " ", "+")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("wellfound: build request: %w", err)
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/129.0.0.0 Safari/537.36")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("wellfound: request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("wellfound: status %d", resp.StatusCode)
	}

	body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("wellfound: read: %w", err)
	}

	// Extract __NEXT_DATA__ JSON from HTML
	jsonStr := extractNextData(string(body))
	if jsonStr == "" {
		return nil, fmt.Errorf("wellfound: __NEXT_DATA__ not found")
	}

	var nd nextData
	if err := json.Unmarshal([]byte(jsonStr), &nd); err != nil {
		return nil, fmt.Errorf("wellfound: decode __NEXT_DATA__: %w", err)
	}

	listings := extractListings(&nd)
	util.Debug("wellfound: parsed listings", map[string]any{"count": len(listings)})

	if len(listings) == 0 {
		return nil, fmt.Errorf("wellfound: no listings found")
	}

	out := make([]model.JobPost, 0, wanted)
	for _, l := range listings {
		if len(out) >= wanted {
			break
		}
		job := mapListing(l)
		if job != nil {
			out = append(out, *job)
		}
	}

	util.Debug("scraper_done", map[string]any{
		"site": s.SiteName(),
		"jobs": len(out),
	})

	if !util.HasMeaningfulJobs(out) {
		return nil, fmt.Errorf("wellfound: no parseable jobs")
	}
	return out, nil
}

// extractNextData finds the __NEXT_DATA__ JSON blob in the HTML page.
func extractNextData(html string) string {
	start := strings.Index(html, `__NEXT_DATA__`)
	if start < 0 {
		return ""
	}
	// Find the opening > after type="application/json"
	gt := strings.Index(html[start:], ">")
	if gt < 0 {
		return ""
	}
	jsonStart := start + gt + 1
	// Find the closing </script>
	end := strings.Index(html[jsonStart:], "</script>")
	if end < 0 {
		return ""
	}
	result := strings.TrimSpace(html[jsonStart : jsonStart+end])
	// Remove HTML comments if any
	result = strings.TrimPrefix(result, "<!--")
	result = strings.TrimSuffix(result, "-->")
	return strings.TrimSpace(result)
}

// extractListings extracts job listings from the __NEXT_DATA__ struct.
func extractListings(nd *nextData) []listing {
	if nd == nil || nd.Props == nil || nd.Props.PageProps == nil {
		return nil
	}
	pp := nd.Props.PageProps
	if len(pp.Listings) > 0 {
		return pp.Listings
	}
	if len(pp.Jobs) > 0 {
		return pp.Jobs
	}
	return nil
}

// mapListing converts a Wellfound listing to a JobPost.
func mapListing(l listing) *model.JobPost {
	if l.Title == "" {
		return nil
	}

	// Build job URL
	slug := l.Slug
	if slug == "" {
		slug = string(l.ID)
	}
	jobURL := "https://wellfound.com/jobs/" + slug

	// Company info
	companyName := ""
	if l.Company != nil && l.Company.Name != "" {
		companyName = l.Company.Name
	}

	companyLogo := ""
	if l.Company != nil && l.Company.LogoURL != "" {
		companyLogo = l.Company.LogoURL
	}

	// Location
	loc := model.Location{}
	if len(l.Locations) > 0 && l.Locations[0] != "" {
		loc.City = l.Locations[0]
	}

	// Compensation
	var comp *model.Compensation
	if l.Compensation != nil && (l.Compensation.Min != nil || l.Compensation.Max != nil) {
		currency := l.Compensation.Currency
		if currency == "" {
			currency = "USD"
		}
		comp = &model.Compensation{
			Interval: model.IntervalYearly,
			Currency: currency,
		}
		if l.Compensation.Min != nil {
			comp.MinAmount = l.Compensation.Min
		}
		if l.Compensation.Max != nil {
			comp.MaxAmount = l.Compensation.Max
		}
	}

	// Date
	var datePosted *time.Time
	if l.CreatedAt != "" {
		if t, err := time.Parse(time.RFC3339, l.CreatedAt); err == nil {
			datePosted = &t
		} else if t, err := time.Parse("2006-01-02T15:04:05Z", l.CreatedAt); err == nil {
			datePosted = &t
		}
	}

	job := &model.JobPost{
		ID:            "wellfound-" + string(l.ID),
		Title:         l.Title,
		CompanyName:   companyName,
		CompanyLogo:   companyLogo,
		JobURL:        jobURL,
		Location:      loc,
		Description:   strings.TrimSpace(l.Description),
		Compensation:  comp,
		IsRemote:      l.Remote,
		DatePosted:    datePosted,
		Site:          string(model.SiteWellfound),
		ApplyMethod:   "external_url",
	}

	return job
}
