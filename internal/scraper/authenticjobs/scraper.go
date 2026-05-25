package authenticjobs

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/util"
)

const apiURL = "https://authenticjobs.com/api/"

// --- API response types ---

type apiResponse struct {
	Listings []apiListing `json:"listings"`
}

type apiListing struct {
	ID          string       `json:"id"`
	Title       string       `json:"title"`
	Company     apiCompany   `json:"company"`
	Description string       `json:"description"`
	Perks       string       `json:"perks"`
	HowToApply  string       `json:"howto_apply"`
	PostDate    string       `json:"post_date"`
	Telecommute string       `json:"telecommuting"`
	Location    apiLocation  `json:"location"`
}

type apiCompany struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type apiLocation struct {
	Name string `json:"name"`
}

// Scraper fetches jobs from the AuthenticJobs API.
type Scraper struct {
	client  *http.Client
	baseURL string
	apiKey  string
}

// New creates a new AuthenticJobs scraper. The API key is read from the
// AUTHENTICJOBS_API_KEY environment variable.
func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Timeout: 20 * time.Second})
	}
	return &Scraper{
		client:  client,
		baseURL: apiURL,
		apiKey:  os.Getenv("AUTHENTICJOBS_API_KEY"),
	}
}

// NewWithURLs creates an AuthenticJobs scraper with explicit configuration. Used for
// testing to point at a local test server with a known API key.
func NewWithURLs(client *http.Client, baseURL, apiKey string) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Timeout: 20 * time.Second})
	}
	return &Scraper{
		client:  client,
		baseURL: baseURL,
		apiKey:  apiKey,
	}
}

// SiteName returns the site identifier.
func (s *Scraper) SiteName() model.Site { return model.SiteAuthenticJobs }

// Scrape fetches jobs from the AuthenticJobs API. It uses page-based pagination
// with a default page size. The API key is sent via the api_key query parameter.
func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{
		"site":           s.SiteName(),
		"results_wanted": input.ResultsWanted,
		"search_term":    input.SearchTerm,
	})

	if s.apiKey == "" {
		return nil, fmt.Errorf("authenticjobs: AUTHENTICJOBS_API_KEY not set")
	}

	wanted := input.ResultsWanted
	if wanted <= 0 {
		wanted = 25
	}

	jobs := make([]model.JobPost, 0, wanted)
	page := 1

	for len(jobs) < wanted {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// Build the request URL with query parameters.
		u, _ := url.Parse(s.baseURL)
		q := u.Query()
		q.Set("api_key", s.apiKey)
		q.Set("page", strconv.Itoa(page))
		if v := strings.TrimSpace(input.SearchTerm); v != "" {
			q.Set("keywords", v)
		}
		u.RawQuery = q.Encode()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
		if err != nil {
			return nil, fmt.Errorf("authenticjobs: build request: %w", err)
		}
		req.Header.Set("Accept", "application/json")

		resp, err := s.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("authenticjobs: request: %w", err)
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			resp.Body.Close()
			return nil, fmt.Errorf("authenticjobs: status %d", resp.StatusCode)
		}

		body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("authenticjobs: read: %w", err)
		}

		var parsed apiResponse
		if err := json.Unmarshal(body, &parsed); err != nil {
			return nil, fmt.Errorf("authenticjobs: decode: %w", err)
		}

		if len(parsed.Listings) == 0 {
			break
		}

		for _, listing := range parsed.Listings {
			if len(jobs) >= wanted {
				break
			}
			job := mapListing(listing)
			if strings.TrimSpace(job.Title) == "" {
				continue
			}
			jobs = append(jobs, job)
		}

		// Continue only if we got results; empty response means no more pages
		if len(parsed.Listings) == 0 {
			break
		}

		page++
	}

	util.Debug("scraper_done", map[string]any{
		"site": s.SiteName(),
		"jobs": len(jobs),
	})

	if !util.HasMeaningfulJobs(jobs) {
		return nil, fmt.Errorf("authenticjobs: no parseable jobs")
	}

	return jobs, nil
}

// mapListing converts a raw AuthenticJobs API listing to a model.JobPost.
func mapListing(l apiListing) model.JobPost {
	job := model.JobPost{
		ID:       "authenticjobs-" + l.ID,
		Title:    strings.TrimSpace(l.Title),
		Site:     string(model.SiteAuthenticJobs),
		IsRemote: strings.ToLower(l.Telecommute) == "yes" || strings.ToLower(l.Telecommute) == "true",
	}

	// Company name
	job.CompanyName = strings.TrimSpace(l.Company.Name)

	// Company URL
	job.CompanyURL = strings.TrimSpace(l.Company.URL)

	// Job URL - AuthenticJobs doesn't provide a direct job URL in the API,
	// but we can construct one from the ID
	if l.ID != "" {
		job.JobURL = fmt.Sprintf("https://authenticjobs.com/jobs/%s", l.ID)
	}

	// Description
	job.Description = strings.TrimSpace(l.Description)

	// Perks
	if perks := strings.TrimSpace(l.Perks); perks != "" {
		job.Description = job.Description + "\n\nPerks: " + perks
	}

	// How to apply
	if howToApply := strings.TrimSpace(l.HowToApply); howToApply != "" {
		job.ApplyMethod = "external_url"
	}

	// Location
	if loc := strings.TrimSpace(l.Location.Name); loc != "" {
		job.Location = model.Location{City: loc}
	}

	// DatePosted from post_date (format: YYYY-MM-DD)
	if l.PostDate != "" {
		if t, err := time.Parse("2006-01-02", l.PostDate); err == nil {
			job.DatePosted = &t
		}
	}

	return job
}