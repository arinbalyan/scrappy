package workable

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

const workableAPIURL = "https://apply.workable.com/api/v1/widget/accounts"

type Scraper struct {
	client *http.Client
	apiURL string
}

func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Timeout: 20 * time.Second})
	}
	return &Scraper{client: client}
}

func NewWithAPIURL(client *http.Client, apiURL string) *Scraper {
	s := New(client)
	if strings.TrimSpace(apiURL) != "" {
		s.apiURL = apiURL
	}
	return s
}

func (s *Scraper) SiteName() model.Site { return model.SiteWorkable }

type wbLocation struct {
	Country     string `json:"country,omitempty"`
	CountryCode string `json:"countryCode,omitempty"`
	City        string `json:"city,omitempty"`
	Region      string `json:"region,omitempty"`
}

type wbJob struct {
	Title          string        `json:"title,omitempty"`
	Shortcode      string        `json:"shortcode,omitempty"`
	EmploymentType string        `json:"employment_type,omitempty"`
	Telecommuting  bool          `json:"telecommuting"`
	Department     string        `json:"department,omitempty"`
	URL            string        `json:"url,omitempty"`
	Shortlink      string        `json:"shortlink,omitempty"`
	ApplicationURL string        `json:"application_url,omitempty"`
	PublishedOn    string        `json:"published_on,omitempty"`
	CreatedAt      string        `json:"created_at,omitempty"`
	City           string        `json:"city,omitempty"`
	State          string        `json:"state,omitempty"`
	Country        string        `json:"country,omitempty"`
	Locations      []wbLocation `json:"locations,omitempty"`
}

type wbResponse struct {
	Jobs []wbJob `json:"jobs"`
}

func (s *Scraper) buildURL(slug string) string {
	if s.apiURL != "" {
		return s.apiURL
	}
	return fmt.Sprintf("%s/%s", workableAPIURL, url.PathEscape(slug))
}

func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{"site": s.SiteName(), "results_wanted": input.ResultsWanted})

	seeds, src := ats.ResolveSeeds(input.SearchTerm, "SCRAPPY_WORKABLE_SEEDS")
	if len(seeds) == 0 {
		return nil, fmt.Errorf("workable no seeds: set SCRAPPY_WORKABLE_SEEDS or pass a company slug in --search")
	}
	util.Debug("workable_seeds", map[string]any{"seeds": seeds, "src": src})

	wanted := input.ResultsWanted
	if wanted <= 0 {
		wanted = 100
	}

	out := make([]model.JobPost, 0, wanted)
	for _, slug := range seeds {
		if len(out) >= wanted {
			break
		}

		u := s.buildURL(slug)
		resp := new(wbResponse)
		if err := ats.FetchJSON(ctx, s.client, u, resp); err != nil {
			util.Warn("workable_fetch_fail", map[string]any{"slug": slug, "err": err.Error()})
			continue
		}

		for _, job := range resp.Jobs {
			if len(out) >= wanted {
				break
			}
			title := strings.TrimSpace(job.Title)
			if title == "" {
				continue
			}

			loc := model.Location{}
			if len(job.Locations) > 0 {
				l := job.Locations[0]
				loc.City = strings.TrimSpace(l.City)
				loc.State = strings.TrimSpace(l.Region)
				loc.Country = strings.TrimSpace(l.Country)
			} else if job.City != "" {
				loc.City = strings.TrimSpace(job.City)
				loc.State = strings.TrimSpace(job.State)
				loc.Country = strings.TrimSpace(job.Country)
			}

			jobURL := strings.TrimSpace(job.URL)
			if jobURL == "" {
				jobURL = strings.TrimSpace(job.Shortlink)
			}
			if jobURL == "" {
				jobURL = fmt.Sprintf("https://apply.workable.com/%s/j/%s", slug, job.Shortcode)
			}

			dep := strings.TrimSpace(job.Department)

			jp := model.JobPost{
				ID:          fmt.Sprintf("workable-%s", job.Shortcode),
				Title:       title,
				CompanyName: slug,
				JobURL:      jobURL,
				Location:    loc,
				IsRemote:    job.Telecommuting,
				Site:        string(model.SiteWorkable),
				Department:  dep,
				JobType:     normalizeEmploymentType(job.EmploymentType),
			}
			dateStr := strings.TrimSpace(job.PublishedOn)
			if dateStr == "" {
				dateStr = strings.TrimSpace(job.CreatedAt)
			}
			if dateStr != "" {
				jp.DatePosted = util.ParseDatePosted(dateStr)
			}
			out = append(out, jp)
		}
	}

	if !util.HasMeaningfulJobs(out) {
		return nil, fmt.Errorf("workable no parseable jobs")
	}
	return out, nil
}

func normalizeEmploymentType(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "fulltime", "full-time", "full_time", "permanent":
		return "fulltime"
	case "parttime", "part-time", "part_time":
		return "parttime"
	case "contract", "contractor", "temporary":
		return "contract"
	case "internship", "intern":
		return "internship"
	}
	return v
}
