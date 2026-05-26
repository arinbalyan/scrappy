package paylocity

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

const paylocityAPIBase = "https://recruiting.paylocity.com/recruiting/api/feed/jobs"

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

func (s *Scraper) SiteName() model.Site { return model.SitePaylocity }

type paylocityJob struct {
	JobID      string `json:"JobId"`
	JobTitle   string `json:"JobTitle"`
	Description string `json:"Description"`
	City       string `json:"City"`
	State      string `json:"State"`
	Country    string `json:"Country"`
	PostedDate  string `json:"PostedDate"`
	JobURL     string `json:"JobUrl"`
	Company    string `json:"Company"`
	Department string `json:"Department"`
	JobType    string `json:"JobType"`
}

func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{"site": s.SiteName(), "results_wanted": input.ResultsWanted})

	seeds, src := ats.ResolveSeeds(input.SearchTerm, "SCRAPPY_PAYLOCITY_SEEDS")
	if len(seeds) == 0 {
		return nil, fmt.Errorf("paylocity no seeds: set SCRAPPY_PAYLOCITY_SEEDS or pass a company slug in --search")
	}
	util.Debug("paylocity_seeds", map[string]any{"seeds": seeds, "src": src})

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

		var jobs []paylocityJob
		if err := ats.FetchJSON(ctx, s.client, u, &jobs); err != nil {
			util.Warn("paylocity_fetch_fail", map[string]any{"slug": slug, "err": err.Error()})
			continue
		}

		for _, job := range jobs {
			if len(out) >= wanted {
				break
			}
			title := strings.TrimSpace(job.JobTitle)
			if title == "" {
				continue
			}
			jp := model.JobPost{
				ID:          "paylocity-" + job.JobID,
				Title:       title,
				CompanyName: nonEmpty(job.Company, slug),
				JobURL:      nonEmpty(job.JobURL, u),
				Location: model.Location{
					City:    strings.TrimSpace(job.City),
					State:   strings.TrimSpace(job.State),
					Country: strings.TrimSpace(job.Country),
				},
				Description:  util.StripHTML(strings.TrimSpace(job.Description)),
				Site:         string(model.SitePaylocity),
				Department:   strings.TrimSpace(job.Department),
				JobType:      normalizeJobType(job.JobType),
			}
			if dp := strings.TrimSpace(job.PostedDate); dp != "" {
				jp.DatePosted = util.ParseDatePosted(dp)
			}
			out = append(out, jp)
		}
	}

	if !util.HasMeaningfulJobs(out) {
		return nil, fmt.Errorf("paylocity no parseable jobs")
	}
	return out, nil
}

func (s *Scraper) buildURL(slug string) string {
	if s.apiURL != "" {
		return s.apiURL
	}
	return fmt.Sprintf("%s/%s", paylocityAPIBase, url.PathEscape(slug))
}

func nonEmpty(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}

func normalizeJobType(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "fulltime", "full-time", "full time", "permanent":
		return "fulltime"
	case "parttime", "part-time", "part time":
		return "parttime"
	case "contract", "contractor", "temporary":
		return "contract"
	case "internship", "intern":
		return "internship"
	}
	return v
}
