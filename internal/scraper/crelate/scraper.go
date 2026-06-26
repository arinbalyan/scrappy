package crelate

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

const crelateAPIURL = "https://app.crelate.com/api3/jobs"

// Scraper fetches jobs from Crelate.
// The company slug is used as the X-Api-Key header.
type Scraper struct {
	client *http.Client
	apiURL string
}

// New creates a new Crelate scraper.
func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Timeout: 20 * time.Second})
	}
	return &Scraper{client: client, apiURL: crelateAPIURL}
}

// NewWithAPIURL creates a new Crelate scraper with a custom API URL (used in tests).
func NewWithAPIURL(client *http.Client, apiURL string) *Scraper {
	s := New(client)
	if strings.TrimSpace(apiURL) != "" {
		s.apiURL = apiURL
	}
	return s
}

func (s *Scraper) SiteName() model.Site { return model.SiteCrelate }

// --- API response types ---

type crelateJob struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	City          string `json:"city"`
	StateProvince string `json:"state_province"`
	PostalCode    string `json:"postal_code"`
	Country       string `json:"country"`
	IsRemote      bool   `json:"is_remote"`
	CreatedDate   string `json:"created_date"`
	ModifiedDate  string `json:"modified_date"`
	Status        string `json:"status"`
}

// Scrape fetches jobs from Crelate.
func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{"site": s.SiteName(), "results_wanted": input.ResultsWanted})

	seeds, src := ats.ResolveSeeds(input.SearchTerm, "SCRAPPY_CRELATE_SEEDS")
	if len(seeds) == 0 {
		return nil, fmt.Errorf("crelate no seeds: set SCRAPPY_CRELATE_SEEDS or pass a company slug in --search")
	}
	if len(seeds) > 8 {
		seeds = seeds[:8]
	}
	util.Debug("crelate_seeds", map[string]any{"seeds": seeds, "src": src})

	wanted := input.ResultsWanted
	if wanted <= 0 {
		wanted = 25
	}

	seen := make(map[string]bool)
	var mu sync.Mutex

	fetchFn := func(ctx context.Context, slug string) ([]model.JobPost, error) {
		u := fmt.Sprintf("%s?published=true&offset=0&limit=%d", s.apiURL, wanted)

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			util.Warn("crelate_request_err", map[string]any{"slug": slug, "err": err.Error()})
			return nil, err
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "Mozilla/5.0")
		req.Header.Set("X-Api-Key", slug)

		resp, err := s.client.Do(req)
		if err != nil {
			util.Warn("crelate_fetch_fail", map[string]any{"slug": slug, "err": err.Error()})
			return nil, err
		}

		body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
		resp.Body.Close()
		if err != nil {
			util.Warn("crelate_read_fail", map[string]any{"slug": slug, "err": err.Error()})
			return nil, err
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			util.Warn("crelate_status", map[string]any{"slug": slug, "status": resp.StatusCode})
			return nil, fmt.Errorf("status %d", resp.StatusCode)
		}

		var apiJobs []crelateJob
		if err := json.Unmarshal(body, &apiJobs); err != nil {
			util.Warn("crelate_decode_fail", map[string]any{"slug": slug, "err": err.Error()})
			return nil, err
		}

		var jobs []model.JobPost
		for _, job := range apiJobs {
			title := strings.TrimSpace(job.Name)
			if title == "" || job.ID == "" {
				continue
			}

			id := ats.BuildID("crelate", slug, job.ID)
			mu.Lock()
			if seen[id] {
				mu.Unlock()
				continue
			}
			seen[id] = true
			mu.Unlock()

			// Location
			l := model.Location{
				City:    strings.TrimSpace(job.City),
				State:   strings.TrimSpace(job.StateProvince),
				Country: strings.TrimSpace(job.Country),
			}

			// Job URL
			jobURL := fmt.Sprintf("https://app.crelate.com/portal/%s/job/%s",
				url.PathEscape(slug), url.PathEscape(job.ID))

			jp := model.JobPost{
				ID:          id,
				Title:       title,
				CompanyName: slug,
				JobURL:      jobURL,
				Location:    l,
				IsRemote:    job.IsRemote,
				Description: util.StripHTML(strings.TrimSpace(job.Description)),
				Site:        string(s.SiteName()),
			}

			if dp := strings.TrimSpace(job.CreatedDate); dp != "" {
				jp.DatePosted = util.ParseDatePosted(dp)
			}

			jobs = append(jobs, jp)
		}
		return jobs, nil
	}

	results := ats.ProcessSeeds(ctx, seeds, 3, wanted, fetchFn)
	if len(results) == 0 {
		return nil, fmt.Errorf("crelate no parseable jobs")
	}
	return results, nil
}
