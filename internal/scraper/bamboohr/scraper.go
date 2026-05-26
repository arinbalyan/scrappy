package bamboohr

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/scraper/ats"
	"github.com/arinbalyan/scrappy/internal/util"
)

const bamboohrAPIURL = "https://%s.bamboohr.com/careers/list"

// Scraper fetches jobs from BambooHR career portals.
type Scraper struct {
	client *http.Client
	apiURL string
}

// New creates a new BambooHR scraper.
func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Timeout: 20 * time.Second})
	}
	return &Scraper{client: client}
}

// NewWithAPIURL creates a new BambooHR scraper with a custom URL (used in tests).
func NewWithAPIURL(client *http.Client, apiURL string) *Scraper {
	s := New(client)
	if strings.TrimSpace(apiURL) != "" {
		s.apiURL = apiURL
	}
	return s
}

func (s *Scraper) SiteName() model.Site { return model.SiteBambooHR }

// --- API response types ---

type bamboohrResponse struct {
	Result []bamboohrJob `json:"result"`
}

type bamboohrLocation struct {
	City    string `json:"city"`
	State   string `json:"state"`
	Country string `json:"country"`
}

type bamboohrJob struct {
	ID                   string            `json:"id"`
	JobOpeningName       string            `json:"jobOpeningName"`
	DepartmentLabel      string            `json:"departmentLabel"`
	Location             *bamboohrLocation `json:"location"`
	EmploymentStatusLabel string           `json:"employmentStatusLabel"`
	MinimumExperience    string            `json:"minimumExperience"`
	Compensation         string            `json:"compensation"`
	Description          string            `json:"description"`
}

// Scrape fetches jobs from BambooHR for the given company seeds.
func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{"site": s.SiteName(), "results_wanted": input.ResultsWanted})

	seeds, src := ats.ResolveSeeds(input.SearchTerm, "SCRAPPY_BAMBOOHR_SEEDS")
	if len(seeds) == 0 {
		return nil, fmt.Errorf("bamboohr no seeds: set SCRAPPY_BAMBOOHR_SEEDS or pass a company slug in --search")
	}
	util.Debug("bamboohr_seeds", map[string]any{"seeds": seeds, "src": src})

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

		var resp bamboohrResponse
		u := s.buildURL(slug)
		if err := ats.FetchJSON(ctx, s.client, u, &resp); err != nil {
			util.Warn("bamboohr_fetch_fail", map[string]any{"slug": slug, "err": err.Error()})
			continue
		}

		for _, job := range resp.Result {
			if len(out) >= wanted {
				break
			}

			title := strings.TrimSpace(job.JobOpeningName)
			if title == "" {
				continue
			}

			id := ats.BuildID("bamboohr", slug, job.ID)
			if seen[id] {
				continue
			}
			seen[id] = true

			// Location
			l := model.Location{}
			if job.Location != nil {
				l.City = strings.TrimSpace(job.Location.City)
				l.State = strings.TrimSpace(job.Location.State)
				l.Country = strings.TrimSpace(job.Location.Country)
			}

			// Job URL
			jobURL := fmt.Sprintf("https://%s.bamboohr.com/careers/%s", url.PathEscape(slug), job.ID)

			jp := model.JobPost{
				ID:          id,
				Title:       title,
				CompanyName: slug,
				JobURL:      jobURL,
				Location:    l,
				Description: util.StripHTML(strings.TrimSpace(job.Description)),
				Site:        string(s.SiteName()),
				Department:  strings.TrimSpace(job.DepartmentLabel),
			}
			out = append(out, jp)
		}
	}

	if !util.HasMeaningfulJobs(out) {
		return nil, fmt.Errorf("bamboohr no parseable jobs")
	}
	return out, nil
}

func (s *Scraper) buildURL(slug string) string {
	if s.apiURL != "" {
		return s.apiURL
	}
	return fmt.Sprintf(bamboohrAPIURL, url.PathEscape(slug))
}
