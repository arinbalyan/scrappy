package trakstar

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

func (s *Scraper) SiteName() model.Site { return model.SiteTrakstar }

type trakstarJob struct {
	ID             int     `json:"id"`
	Title          string  `json:"title"`
	Description    string  `json:"description,omitempty"`
	Department     string  `json:"department,omitempty"`
	Location       string  `json:"location,omitempty"`
	City           string  `json:"city,omitempty"`
	State          string  `json:"state,omitempty"`
	Country        string  `json:"country,omitempty"`
	EmploymentType string  `json:"employment_type,omitempty"`
	URL            string  `json:"url,omitempty"`
	ApplyURL       string  `json:"apply_url,omitempty"`
	CreatedAt      string  `json:"created_at,omitempty"`
	Remote         *bool   `json:"remote,omitempty"`
	SalaryMin      float64 `json:"salary_min,omitempty"`
	SalaryMax      float64 `json:"salary_max,omitempty"`
	SalaryCurrency string  `json:"salary_currency,omitempty"`
}

func (s *Scraper) buildURL(slug string) string {
	if s.apiURL != "" {
		return s.apiURL
	}
	return fmt.Sprintf("https://%s.hire.trakstar.com/api/v1/openings", url.PathEscape(slug))
}

func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{"site": s.SiteName(), "results_wanted": input.ResultsWanted})

	seeds, src := ats.ResolveSeeds(input.SearchTerm, "SCRAPPY_TRAKSTAR_SEEDS")
	if len(seeds) == 0 {
		return nil, fmt.Errorf("trakstar no seeds: set SCRAPPY_TRAKSTAR_SEEDS or pass a company slug in --search")
	}
	util.Debug("trakstar_seeds", map[string]any{"seeds": seeds, "src": src})

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
		var jobs []trakstarJob
		if err := ats.FetchJSON(ctx, s.client, u, &jobs); err != nil {
			util.Warn("trakstar_fetch_fail", map[string]any{"slug": slug, "err": err.Error()})
			continue
		}

		for _, job := range jobs {
			if len(out) >= wanted {
				break
			}
			title := strings.TrimSpace(job.Title)
			if title == "" {
				continue
			}

			loc := model.Location{
				City:    strings.TrimSpace(job.City),
				State:   strings.TrimSpace(job.State),
				Country: strings.TrimSpace(job.Country),
			}
			if job.City == "" && job.State == "" && job.Country == "" && job.Location != "" {
				loc.City = strings.TrimSpace(job.Location)
			}

			isRemote := false
			if job.Remote != nil && *job.Remote {
				isRemote = true
			}

			jobURL := strings.TrimSpace(job.URL)
			if jobURL == "" {
				jobURL = fmt.Sprintf("https://%s.hire.trakstar.com/openings/%d", slug, job.ID)
			}

			jp := model.JobPost{
				ID:          fmt.Sprintf("trakstar-%d", job.ID),
				Title:       title,
				CompanyName: slug,
				JobURL:      jobURL,
				Location:    loc,
				IsRemote:    isRemote,
				Description: util.StripHTML(strings.TrimSpace(job.Description)),
				Site:        string(model.SiteTrakstar),
				Department:  strings.TrimSpace(job.Department),
				JobType:     strings.TrimSpace(job.EmploymentType),
			}

			if job.SalaryMin != 0 || job.SalaryMax != 0 {
				minAmt := job.SalaryMin
				maxAmt := job.SalaryMax
				currency := job.SalaryCurrency
				if currency == "" {
					currency = "USD"
				}
				jp.Compensation = &model.Compensation{
					Interval:  model.IntervalYearly,
					MinAmount: &minAmt,
					MaxAmount: &maxAmt,
					Currency:  currency,
				}
			}

			if job.CreatedAt != "" {
				jp.DatePosted = util.ParseDatePosted(job.CreatedAt)
			}

			out = append(out, jp)
		}
	}

	if !util.HasMeaningfulJobs(out) {
		return nil, fmt.Errorf("trakstar no parseable jobs")
	}
	return out, nil
}
