package wellfound

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/util"
)

const (
	searchURL      = "https://wellfound.com/jobs"
	maxPages       = 3
	defaultWanted  = 15
)

var (
	// nextDataRe matches the __NEXT_DATA__ JSON blob embedded in the page.
	nextDataRe = regexp.MustCompile(`(?is)<script\s+id="__NEXT_DATA__"\s+type="application/json"[^>]*>(.*?)</script>`)
	stripHTMLRe = regexp.MustCompile(`(?is)<[^>]+>`)
)

// ---- Wellfound API types ----

type nextData struct {
	Props *pageProps `json:"props,omitempty"`
}

type pageProps struct {
	PageProps *innerProps `json:"pageProps,omitempty"`
}

type innerProps struct {
	Listings []listing `json:"listings,omitempty"`
	Jobs     []listing `json:"jobs,omitempty"`
}

type listing struct {
	ID        any        `json:"id"`
	Title     string     `json:"title"`
	Slug      string     `json:"slug,omitempty"`
	Company   *company   `json:"company,omitempty"`
	Comp      *compField `json:"compensation,omitempty"`
	Locations []string   `json:"locations,omitempty"`
	Remote    bool       `json:"remote,omitempty"`
	Desc      string     `json:"description,omitempty"`
	Skills    []string   `json:"skills,omitempty"`
	CreatedAt string     `json:"createdAt,omitempty"`
}

type company struct {
	Name     string `json:"name,omitempty"`
	Slug     string `json:"slug,omitempty"`
	LogoURL  string `json:"logoUrl,omitempty"`
}

type compField struct {
	Min      *float64 `json:"min,omitempty"`
	Max      *float64 `json:"max,omitempty"`
	Currency string   `json:"currency,omitempty"`
}

// Scraper fetches jobs from Wellfound by scraping the embedded __NEXT_DATA__.
type Scraper struct {
	client  *http.Client
	baseURL string
}

// New creates a new Wellfound scraper.
func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{
			Retries: 2,
			Timeout: 20 * time.Second,
			UserAgents: []string{
				"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/129.0.0.0 Safari/537.36",
			},
		})
	}
	return &Scraper{client: client, baseURL: searchURL}
}

// NewWithBaseURL creates a new scraper with a custom endpoint (used in tests).
func NewWithBaseURL(client *http.Client, baseURL string) *Scraper {
	s := New(client)
	if strings.TrimSpace(baseURL) != "" {
		s.baseURL = strings.TrimSpace(baseURL)
	}
	return s
}

// SiteName returns the site identifier.
func (s *Scraper) SiteName() model.Site { return model.SiteWellfound }

// Scrape fetches jobs from Wellfound.
func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{
		"site":           s.SiteName(),
		"results_wanted": input.ResultsWanted,
		"search_term":    input.SearchTerm,
	})

	wanted := input.ResultsWanted
	if wanted <= 0 {
		wanted = defaultWanted
	}

	term := strings.TrimSpace(input.SearchTerm)

	jobs := make([]model.JobPost, 0, wanted)

	for page := 1; page <= maxPages && len(jobs) < wanted; page++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		url := s.baseURL
		q := urlQueryParams(term, page)
		if q != "" {
			url = s.baseURL + "?" + q
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("wellfound: build request: %w", err)
		}
		req.Header.Set("Accept", "text/html,application/xhtml+xml")
		req.Header.Set("Accept-Language", "en-US,en;q=0.9")
		req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/129.0.0.0 Safari/537.36")

		resp, err := s.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("wellfound: request: %w", err)
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			resp.Body.Close()
			return nil, fmt.Errorf("wellfound: status %d", resp.StatusCode)
		}

		body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("wellfound: read: %w", err)
		}

		listings, err := extractListings(body)
		if err != nil || len(listings) == 0 {
			if len(listings) == 0 && err == nil {
				break
			}
			break
		}

		for _, l := range listings {
			if len(jobs) >= wanted {
				break
			}
			job := mapListing(l)
			job.Site = string(s.SiteName())
			jobs = append(jobs, job)
		}
	}

	util.Debug("scraper_done", map[string]any{
		"site": s.SiteName(),
		"jobs": len(jobs),
	})

	if !util.HasMeaningfulJobs(jobs) {
		return nil, fmt.Errorf("wellfound: no parseable jobs")
	}
	return jobs, nil
}

// extractListings extracts job listings from __NEXT_DATA__ embedded in HTML.
func extractListings(body []byte) ([]listing, error) {
	match := nextDataRe.FindSubmatch(body)
	if len(match) < 2 {
		// Check for anti-bot challenge
		if util.DetectAntiBotChallenge(body) != "" {
			return nil, fmt.Errorf("anti-bot challenge detected")
		}
		return nil, fmt.Errorf("__NEXT_DATA__ not found")
	}

	var nd nextData
	if err := json.Unmarshal(match[1], &nd); err != nil {
		return nil, fmt.Errorf("parse __NEXT_DATA__: %w", err)
	}

	if nd.Props == nil || nd.Props.PageProps == nil {
		return nil, nil
	}

	listings := nd.Props.PageProps.Listings
	if len(listings) == 0 {
		listings = nd.Props.PageProps.Jobs
	}
	return listings, nil
}

// mapListing converts a Wellfound listing to a JobPost.
func mapListing(l listing) model.JobPost {
	jobID := fmt.Sprintf("%v", l.ID)
	slug := l.Slug
	if slug == "" {
		slug = jobID
	}

	jobURL := fmt.Sprintf("https://wellfound.com/jobs/%s", slug)

	// Company name
	companyName := ""
	if l.Company != nil {
		companyName = l.Company.Name
	}

	// Description (strip HTML)
	desc := strings.TrimSpace(stripHTMLRe.ReplaceAllString(l.Desc, " "))

	// Location
	location := model.Location{}
	if len(l.Locations) > 0 {
		location.City = l.Locations[0]
	}

	// Compensation
	var comp *model.Compensation
	if l.Comp != nil && (l.Comp.Min != nil || l.Comp.Max != nil) {
		curr := l.Comp.Currency
		if curr == "" {
			curr = "USD"
		}
		comp = &model.Compensation{
			Interval: "yearly",
			MinAmount: l.Comp.Min,
			MaxAmount: l.Comp.Max,
			Currency:  curr,
		}
	}

	// Date
	var datePosted *time.Time
	if l.CreatedAt != "" {
		datePosted = parseDate(l.CreatedAt)
	}

	return model.JobPost{
		ID:           "wellfound-" + jobID,
		Title:        l.Title,
		CompanyName:  companyName,
		CompanyLogoURL: companyLogoURL(l),
		JobURL:       jobURL,
		Location:     location,
		IsRemote:     l.Remote,
		Description:  desc,
		Compensation: comp,
		DatePosted:   datePosted,
		ApplyMethod:  "external_url",
		Skills:       l.Skills,
	}
}

// companyLogoURL returns the company logo URL.
func companyLogoURL(l listing) string {
	if l.Company != nil {
		return l.Company.LogoURL
	}
	return ""
}

// urlQueryParams builds query parameters for Wellfound.
func urlQueryParams(searchTerm string, page int) string {
	params := make([]string, 0, 2)
	if searchTerm != "" {
		params = append(params, "q="+strings.ReplaceAll(searchTerm, " ", "+"))
	}
	return strings.Join(params, "&")
}

// parseDate parses a date string.
func parseDate(v string) *time.Time {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05",
		"2006-01-02T15:04:05.000Z",
		"2006-01-02",
	}
	for _, f := range formats {
		t, err := time.Parse(f, v)
		if err == nil {
			return &t
		}
	}
	return nil
}
