package breezyhr

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

// Scraper fetches jobs from BreezyHR career portals.
type Scraper struct {
	client *http.Client
	apiURL string
}

// New creates a new BreezyHR scraper.
func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Timeout: 20 * time.Second})
	}
	return &Scraper{client: client}
}

// NewWithAPIURL creates a new BreezyHR scraper with a custom URL (used in tests).
func NewWithAPIURL(client *http.Client, apiURL string) *Scraper {
	s := New(client)
	if strings.TrimSpace(apiURL) != "" {
		s.apiURL = apiURL
	}
	return s
}

func (s *Scraper) SiteName() model.Site { return model.SiteBreezyHR }

// --- API response types ---

type breezyLocation struct {
	City     string `json:"city"`
	State    string `json:"state"`
	Country  string `json:"country"`
	IsRemote bool   `json:"is_remote"`
}

type breezyListing struct {
	ID          string          `json:"id"`
	FriendlyID  string          `json:"friendly_id"`
	Name        string          `json:"name"`
	Title       string          `json:"title"`
	Description string          `json:"description"`
	URL         string          `json:"url"`
	Location    *breezyLocation `json:"location"`
	Department  string          `json:"department"`
	Category    *struct {
		Name string `json:"name"`
	} `json:"category"`
	PublishedDate string `json:"published_date"`
	CreationDate  string `json:"creation_date"`
}

// Scrape fetches jobs from BreezyHR.
func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{"site": s.SiteName(), "results_wanted": input.ResultsWanted})

	seeds, src := ats.ResolveSeeds(input.SearchTerm, "SCRAPPY_BREEZYHR_SEEDS")
	if len(seeds) == 0 {
		return nil, fmt.Errorf("breezyhr no seeds: set SCRAPPY_BREEZYHR_SEEDS or pass a company slug in --search")
	}
	util.Debug("breezyhr_seeds", map[string]any{"seeds": seeds, "src": src})

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
			util.Warn("breezyhr_request_err", map[string]any{"slug": slug, "err": err.Error()})
			continue
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "Mozilla/5.0")

		resp, err := s.client.Do(req)
		if err != nil {
			util.Warn("breezyhr_fetch_fail", map[string]any{"slug": slug, "err": err.Error()})
			continue
		}

		body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
		resp.Body.Close()
		if err != nil {
			util.Warn("breezyhr_read_fail", map[string]any{"slug": slug, "err": err.Error()})
			continue
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			util.Warn("breezyhr_status", map[string]any{"slug": slug, "status": resp.StatusCode})
			continue
		}

		var listings []breezyListing
		if err := json.Unmarshal(body, &listings); err != nil {
			util.Warn("breezyhr_decode_fail", map[string]any{"slug": slug, "err": err.Error()})
			continue
		}

		for _, listing := range listings {
			if len(out) >= wanted {
				break
			}

			title := strings.TrimSpace(listing.Name)
			if title == "" {
				title = strings.TrimSpace(listing.Title)
			}
			if title == "" {
				continue
			}

			jobID := listing.ID
			if jobID == "" {
				jobID = listing.FriendlyID
			}
			if jobID == "" {
				continue
			}

			id := ats.BuildID("breezyhr", slug, jobID)
			if seen[id] {
				continue
			}
			seen[id] = true

			// Location
			l := model.Location{}
			isRemote := false
			if listing.Location != nil {
				l.City = strings.TrimSpace(listing.Location.City)
				l.State = strings.TrimSpace(listing.Location.State)
				l.Country = strings.TrimSpace(listing.Location.Country)
				isRemote = listing.Location.IsRemote
			}

			// Job URL
			jobURL := strings.TrimSpace(listing.URL)
			if jobURL == "" {
				fid := listing.FriendlyID
				if fid == "" {
					fid = listing.ID
				}
				jobURL = fmt.Sprintf("https://%s.breezy.hr/p/%s", slug, fid)
			}

			// Department
			dept := strings.TrimSpace(listing.Department)
			if dept == "" && listing.Category != nil {
				dept = strings.TrimSpace(listing.Category.Name)
			}

			jp := model.JobPost{
				ID:          id,
				Title:       title,
				CompanyName: slug,
				JobURL:      jobURL,
				Location:    l,
				IsRemote:    isRemote,
				Description: util.StripHTML(strings.TrimSpace(listing.Description)),
				Site:        string(s.SiteName()),
				Department:  dept,
			}

			if dp := strings.TrimSpace(listing.PublishedDate); dp != "" {
				jp.DatePosted = util.ParseDatePosted(dp)
			} else if cd := strings.TrimSpace(listing.CreationDate); cd != "" {
				jp.DatePosted = util.ParseDatePosted(cd)
			}

			out = append(out, jp)
		}
	}

	if !util.HasMeaningfulJobs(out) {
		return nil, fmt.Errorf("breezyhr no parseable jobs")
	}
	return out, nil
}

func (s *Scraper) buildURL(slug string) string {
	if s.apiURL != "" {
		return s.apiURL
	}
	return fmt.Sprintf("https://%s.breezy.hr/json", slug)
}
