package hiringcafe

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/util"
)

const apiEndpoint = "https://hiring.cafe/api/search-jobs"

type Scraper struct {
	client *http.Client
	apiURL string
}

func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Retries: 3, Timeout: 25 * time.Second})
	}
	return &Scraper{client: client, apiURL: apiEndpoint}
}
func NewWithAPIURL(client *http.Client, endpoint string) *Scraper {
	s := New(client)
	if strings.TrimSpace(endpoint) != "" {
		s.apiURL = endpoint
	}
	return s
}
func (s *Scraper) SiteName() model.Site { return model.SiteHiringCafe }

// searchPayload is the JSON body sent to the Hiring Cafe search API.
type searchPayload struct {
	Size        int         `json:"size"`
	Page        int         `json:"page"`
	SearchState searchState `json:"searchState"`
}

type searchState struct {
	Locations            []locationFilter `json:"locations"`
	WorkplaceTypes       []string         `json:"workplaceTypes"`
	SearchQuery          string           `json:"searchQuery"`
	DateFetchedPastNDays int              `json:"dateFetchedPastNDays"`
	SortBy               string           `json:"sortBy"`
	SeniorityLevel       []string         `json:"seniorityLevel,omitempty"`
}

type locationFilter struct {
	FormattedAddress string `json:"formatted_address,omitempty"`
}

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

	// Build the search state payload
	payload := searchPayload{
		Size: wanted,
		Page: 0,
		SearchState: searchState{
			Locations: []locationFilter{
				{FormattedAddress: "United States"},
			},
			WorkplaceTypes:       []string{"Remote", "Hybrid", "Onsite"},
			SearchQuery:          strings.TrimSpace(input.SearchTerm),
			DateFetchedPastNDays: 30,
			SortBy:               "default",
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("hiringcafe marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.apiURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("hiringcafe request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")
	req.Header.Set("Origin", "https://hiring.cafe")
	req.Header.Set("Referer", "https://hiring.cafe/")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("hiringcafe request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("hiringcafe status %d", resp.StatusCode)
	}

	rawBody, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("hiringcafe read: %w", err)
	}

	// Parse the response. The API can return jobs in multiple formats:
	//   1. Direct JSON array of job objects
	//   2. JSON object with a "jobs", "results", "data", "items", or "content" key
	//   3. Elasticsearch-style response with "hits"
	jobs := parseAPIResponse(rawBody, wanted)
	if !util.HasMeaningfulJobs(jobs) {
		return nil, fmt.Errorf("hiringcafe no parseable jobs")
	}
	util.Debug("scraper_done", map[string]any{
		"site": s.SiteName(),
		"jobs": len(jobs),
	})
	return jobs, nil
}

// parseAPIResponse flexibly parses the Hiring Cafe API response.
func parseAPIResponse(raw []byte, wanted int) []model.JobPost {
	// Try parsing as a direct array of flat job maps
	var direct []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &direct); err == nil && len(direct) > 0 {
		return mapFlatJobs(direct, wanted)
	}

	// Try parsing as a JSON object with various possible job array keys
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil
	}

	for _, key := range []string{"jobs", "results", "data", "items", "content"} {
		if rawArr, ok := obj[key]; ok {
			var arr []map[string]json.RawMessage
			if json.Unmarshal(rawArr, &arr) == nil && len(arr) > 0 {
				return mapFlatJobs(arr, wanted)
			}
		}
	}

	// Try Elasticsearch-style response
	if hitsRaw, ok := obj["hits"]; ok {
		var hits struct {
			Hits []struct {
				Source map[string]json.RawMessage `json:"_source"`
			} `json:"hits"`
		}
		if json.Unmarshal(hitsRaw, &hits) == nil && len(hits.Hits) > 0 {
			sources := make([]map[string]json.RawMessage, len(hits.Hits))
			for i, h := range hits.Hits {
				sources[i] = h.Source
			}
			return mapFlatJobs(sources, wanted)
		}
	}

	return nil
}

// mapFlatJobs converts a slice of raw job maps to JobPost.
// Handles multiple field naming conventions via case-insensitive match.
func mapFlatJobs(raw []map[string]json.RawMessage, wanted int) []model.JobPost {
	if wanted <= 0 || wanted > len(raw) {
		wanted = len(raw)
	}
	out := make([]model.JobPost, 0, wanted)
	for i, item := range raw {
		if len(out) >= wanted {
			break
		}
		var f flatJob
		flatJobFromMap(item, &f)
		title := strings.TrimSpace(f.Title)
		company := strings.TrimSpace(f.Company)
		if title == "" {
			continue
		}
		// Allow empty company for some API formats; use title as placeholder
		if company == "" {
			company = title // ponytail: keep job even without company
		}
		post := model.JobPost{
			ID:          "hc-" + strings.TrimSpace(f.ID),
			Title:       title,
			CompanyName: company,
			JobURL:      strings.TrimSpace(f.URL),
			Description: strings.TrimSpace(f.Description),
			Location:    model.Location{City: strings.TrimSpace(f.Location)},
			IsRemote:    f.Remote || strings.Contains(strings.ToLower(f.Location), "remote"),
		}
		if post.ID == "hc-" {
			post.ID = fmt.Sprintf("hc-%d", i+1)
		}
		if post.JobURL == "" {
			post.JobURL = apiEndpoint
		}
		if f.PostedAt != "" {
			if t, err := time.Parse(time.RFC3339, f.PostedAt); err == nil {
				post.DatePosted = &t
			} else if t, err := time.Parse("2006-01-02T15:04:05Z", f.PostedAt); err == nil {
				post.DatePosted = &t
			} else if t, err := time.Parse("2006-01-02", f.PostedAt); err == nil {
				post.DatePosted = &t
			}
		}
		out = append(out, post)
	}
	return out
}

// flatJob is populated by flatJobFromMap, which extracts fields from a
// map[string]json.RawMessage by matching field names case-insensitively.
type flatJob struct {
	ID          string
	Title       string
	Company     string
	URL         string
	Location    string
	Description string
	PostedAt    string
	Remote      bool
}

// flatJobFromMap populates f from a raw map, trying various field name
// conventions (camelCase, snake_case, different casing).
func flatJobFromMap(m map[string]json.RawMessage, f *flatJob) {
	// Helper to get string value for a key, checking multiple aliases
	getStr := func(keys ...string) string {
		for _, k := range keys {
			if raw, ok := m[k]; ok {
				var s string
				if json.Unmarshal(raw, &s) == nil && s != "" {
					return s
				}
			}
		}
		// Case-insensitive fallback
		for k, raw := range m {
			for _, target := range keys {
				if strings.EqualFold(k, target) {
					var s string
					if json.Unmarshal(raw, &s) == nil && s != "" {
						return s
					}
				}
			}
		}
		return ""
	}

	f.ID = getStr("id", "ID", "jobId", "job_id")
	f.Title = getStr("title", "Title", "jobTitle", "job_title", "name", "Name")
	f.Company = getStr("company", "Company", "companyName", "company_name", "companyNameAlt",
		"employer", "Employer", "source", "Source")
	f.URL = getStr("url", "URL", "applyUrl", "apply_url", "ApplyURL", "detailUrl", "detail_url")
	f.Location = getStr("location", "Location", "formatted_workplace_location", "city", "City")
	f.Description = getStr("description", "Description", "snippet", "Snippet")
	f.PostedAt = getStr("posted_at", "postedAt", "PostedAt", "PostDate", "postDate",
		"published_at", "publishedAt", "date_posted", "datePosted", "DatePosted")

	// Try bool Remote field
	for _, k := range []string{"remote", "Remote", "isRemote", "is_remote"} {
		if raw, ok := m[k]; ok {
			var b bool
			if json.Unmarshal(raw, &b) == nil {
				f.Remote = b
				break
			}
			var s string
			if json.Unmarshal(raw, &s) == nil {
				f.Remote = strings.EqualFold(s, "true") || s == "1"
				break
			}
		}
	}
}
