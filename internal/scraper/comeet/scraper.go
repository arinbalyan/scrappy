package comeet

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/scraper/ats"
	"github.com/arinbalyan/scrappy/internal/util"
)

// Scraper fetches jobs from Comeet career portals.
type Scraper struct {
	client *http.Client
	apiURL string
}

// New creates a new Comeet scraper.
func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Timeout: 20 * time.Second})
	}
	return &Scraper{client: client}
}

// NewWithAPIURL creates a new Comeet scraper with a custom URL (used in tests).
func NewWithAPIURL(client *http.Client, apiURL string) *Scraper {
	s := New(client)
	if strings.TrimSpace(apiURL) != "" {
		s.apiURL = apiURL
	}
	return s
}

func (s *Scraper) SiteName() model.Site { return model.SiteComeet }

// --- API response types ---

type comeetLocation struct {
	Name string `json:"name"`
}

type comeetDetail struct {
	Value string `json:"value"`
}

type comeetListing struct {
	UID          string            `json:"uid"`
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	CompanyName  string            `json:"company_name"`
	URL          string            `json:"url"`
	URLActivePage string           `json:"url_active_page"`
	Location     *comeetLocation   `json:"location"`
	Details      []comeetDetail    `json:"details"`
	Department   string            `json:"department"`
	TimeUpdated  string            `json:"time_updated"`
}

// Scrape fetches jobs from Comeet.
func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{"site": s.SiteName(), "results_wanted": input.ResultsWanted})

	seeds, src := ats.ResolveSeeds(input.SearchTerm, "SCRAPPY_COMEET_SEEDS")
	if len(seeds) == 0 {
		return nil, fmt.Errorf("comeet no seeds: set SCRAPPY_COMEET_SEEDS or pass a company slug in --search")
	}
	util.Debug("comeet_seeds", map[string]any{"seeds": seeds, "src": src})

	wanted := input.ResultsWanted
	if wanted <= 0 {
		wanted = 25
	}

	out := make([]model.JobPost, 0, wanted)
	seen := make(map[string]bool)

	for _, slug := range seeds {
		if len(out) >= wanted {
			break
		}

		u := s.buildURL(slug)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			util.Warn("comeet_request_err", map[string]any{"slug": slug, "err": err.Error()})
			continue
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "Mozilla/5.0")

		resp, err := s.client.Do(req)
		if err != nil {
			util.Warn("comeet_fetch_fail", map[string]any{"slug": slug, "err": err.Error()})
			continue
		}

		body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
		resp.Body.Close()
		if err != nil {
			util.Warn("comeet_read_fail", map[string]any{"slug": slug, "err": err.Error()})
			continue
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			util.Warn("comeet_status", map[string]any{"slug": slug, "status": resp.StatusCode})
			continue
		}

		var listings []comeetListing
		if err := json.Unmarshal(body, &listings); err != nil {
			util.Warn("comeet_decode_fail", map[string]any{"slug": slug, "err": err.Error()})
			continue
		}

		for _, listing := range listings {
			if len(out) >= wanted {
				break
			}

			title := strings.TrimSpace(listing.Name)
			if title == "" {
				continue
			}

			jobID := listing.UID
			if jobID == "" {
				jobID = listing.ID
			}
			if jobID == "" {
				continue
			}

			id := ats.BuildID("comeet", slug, jobID)
			if seen[id] {
				continue
			}
			seen[id] = true

			// Location
			l := model.Location{}
			isRemote := false
			if listing.Location != nil {
				locStr := strings.TrimSpace(listing.Location.Name)
				if strings.Contains(strings.ToLower(locStr), "remote") {
					isRemote = true
				}
				l.City = locStr
			}

			// Job URL
			jobURL := strings.TrimSpace(listing.URLActivePage)
			if jobURL == "" {
				jobURL = strings.TrimSpace(listing.URL)
			}

			// Description from details
			desc := ""
			for _, d := range listing.Details {
				desc += strings.TrimSpace(d.Value) + "\n"
			}
			desc = strings.TrimSpace(desc)

			jp := model.JobPost{
				ID:          id,
				Title:       title,
				CompanyName: strings.TrimSpace(listing.CompanyName),
				JobURL:      jobURL,
				Location:    l,
				IsRemote:    isRemote,
				Description: util.StripHTML(desc),
				Site:        string(s.SiteName()),
				Department:  strings.TrimSpace(listing.Department),
			}

			if jp.CompanyName == "" {
				jp.CompanyName = slug
			}

			if dp := strings.TrimSpace(listing.TimeUpdated); dp != "" {
				jp.DatePosted = util.ParseDatePosted(dp)
			}

			out = append(out, jp)
		}
	}

	if !util.HasMeaningfulJobs(out) {
		return nil, fmt.Errorf("comeet no parseable jobs")
	}
	return out, nil
}

func (s *Scraper) buildURL(slug string) string {
	if s.apiURL != "" {
		return s.apiURL
	}
	return fmt.Sprintf("https://www.comeet.com/careers-api/2.0/company/%s/positions?token=", slug)
}
