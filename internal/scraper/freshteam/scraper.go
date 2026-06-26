package freshteam

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/scraper/ats"
	"github.com/arinbalyan/scrappy/internal/util"
)

// Scraper fetches jobs from Freshteam.
// Requires FRESHTEAM_API_KEY env var for authentication and SCRAPPY_FRESHTEAM_SEEDS
// for company slugs, or pass a single slug in --search.
type Scraper struct {
	client *http.Client
	apiURL string
}

// New creates a new Freshteam scraper.
func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Timeout: 20 * time.Second})
	}
	return &Scraper{client: client}
}

// NewWithAPIURL creates a new Freshteam scraper with a custom API URL (used in tests).
func NewWithAPIURL(client *http.Client, apiURL string) *Scraper {
	s := New(client)
	if strings.TrimSpace(apiURL) != "" {
		s.apiURL = apiURL
	}
	return s
}

func (s *Scraper) SiteName() model.Site { return model.SiteFreshteam }

// --- API response types ---

type freshteamJobPosting struct {
	ID                int    `json:"id"`
	Title             string `json:"title"`
	Description       string `json:"description"`
	Department        string `json:"department"`
	Branch            string `json:"branch"`
	Type              string `json:"type"`
	Remote            bool   `json:"remote"`
	ClosingDate       string `json:"closing_date"`
	CreatedAt         string `json:"created_at"`
	ApplicantApplyLink string `json:"applicant_apply_link"`
}

// Scrape fetches jobs from Freshteam.
func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{"site": s.SiteName(), "results_wanted": input.ResultsWanted})

	seeds, src := ats.ResolveSeeds(input.SearchTerm, "SCRAPPY_FRESHTEAM_SEEDS")
	if len(seeds) == 0 {
		return nil, fmt.Errorf("freshteam no seeds: set SCRAPPY_FRESHTEAM_SEEDS or pass a company slug in --search")
	}
	// If the search term was used as slug and it looks like a search phrase,
	// return early — Freshteam needs a company slug, not a search string.
	if src == ats.SeedFromSearch && (strings.ContainsAny(seeds[0], " \"") || strings.Contains(seeds[0], "OR")) {
		return nil, fmt.Errorf("freshteam: no tenant slugs — got search term %q; set SCRAPPY_FRESHTEAM_SEEDS or pass --search 'company-slug'", seeds[0])
	}
	util.Debug("freshteam_seeds", map[string]any{"seeds": seeds, "src": src})

	// Freshteam requires an API key
	apiKey := ""
	if len(seeds) > 1 {
		// Use last seed as potential key — but really it comes from env
		_ = seeds[len(seeds)-1]
	}
	// First seed is the company slug; we need API key from seeds too or env
	if strings.Contains(seeds[0], ":") {
		// Format: slug:apiKey
		parts := strings.SplitN(seeds[0], ":", 2)
		seeds = []string{parts[0]}
		apiKey = parts[1]
	}

	wanted := input.ResultsWanted
	if wanted <= 0 {
		wanted = 25
	}

	seen := make(map[string]bool)
	var mu sync.Mutex

	fetchFn := func(ctx context.Context, slug string) ([]model.JobPost, error) {
		u := s.buildURL(slug)

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			util.Warn("freshteam_request_err", map[string]any{"slug": slug, "err": err.Error()})
			return nil, err
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "Mozilla/5.0")
		if apiKey != "" {
			req.Header.Set("Authorization", "Bearer "+apiKey)
		}

		resp, err := s.client.Do(req)
		if err != nil {
			util.Warn("freshteam_fetch_fail", map[string]any{"slug": slug, "err": err.Error()})
			return nil, err
		}

		body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
		resp.Body.Close()
		if err != nil {
			util.Warn("freshteam_read_fail", map[string]any{"slug": slug, "err": err.Error()})
			return nil, err
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			util.Warn("freshteam_status", map[string]any{"slug": slug, "status": resp.StatusCode})
			return nil, fmt.Errorf("status %d", resp.StatusCode)
		}

		var postings []freshteamJobPosting
		if err := json.Unmarshal(body, &postings); err != nil {
			util.Warn("freshteam_decode_fail", map[string]any{"slug": slug, "err": err.Error()})
			return nil, err
		}

		var jobs []model.JobPost
		for _, posting := range postings {
			title := strings.TrimSpace(posting.Title)
			if title == "" {
				continue
			}

			id := ats.BuildID("freshteam", slug, fmt.Sprintf("%d", posting.ID))
			mu.Lock()
			if seen[id] {
				mu.Unlock()
				continue
			}
			seen[id] = true
			mu.Unlock()

			// Location from branch field
			l := model.Location{}
			branch := strings.TrimSpace(posting.Branch)
			if strings.Contains(strings.ToLower(branch), "remote") {
				_ = true // isRemote will be set from posting.Remote
			}
			l.City = branch

			// Job URL
			jobURL := strings.TrimSpace(posting.ApplicantApplyLink)
			if jobURL == "" {
				jobURL = fmt.Sprintf("https://%s.freshteam.com/jobs/%d", url.PathEscape(slug), posting.ID)
			}

			jp := model.JobPost{
				ID:          id,
				Title:       title,
				CompanyName: slug,
				JobURL:      jobURL,
				Location:    l,
				IsRemote:    posting.Remote,
				Description: util.StripHTML(strings.TrimSpace(posting.Description)),
				Site:        string(s.SiteName()),
				Department:  strings.TrimSpace(posting.Department),
				JobType:     normalizeEmploymentType(posting.Type),
			}

			if dp := strings.TrimSpace(posting.CreatedAt); dp != "" {
				jp.DatePosted = util.ParseDatePosted(dp)
			}

			jobs = append(jobs, jp)
		}
		return jobs, nil
	}

	results := ats.ProcessSeeds(ctx, seeds, 3, wanted, fetchFn)
	if len(results) == 0 {
		return nil, fmt.Errorf("freshteam no parseable jobs")
	}
	return results, nil
}

func (s *Scraper) buildURL(slug string) string {
	if s.apiURL != "" {
		return s.apiURL
	}
	return fmt.Sprintf("https://%s.freshteam.com/api/job_postings", url.PathEscape(slug))
}

func normalizeEmploymentType(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "fulltime", "full-time", "permanent":
		return "fulltime"
	case "parttime", "part-time":
		return "parttime"
	case "contract", "contractor", "temporary":
		return "contract"
	case "internship", "intern":
		return "internship"
	}
	return v
}
