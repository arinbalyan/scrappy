package recruitee

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

func (s *Scraper) SiteName() model.Site { return model.SiteRecruitee }

type recruiteeResponse struct {
	Offers []recruiteeOffer `json:"offers"`
}

type recruiteeOffer struct {
	ID            int     `json:"id"`
	Title         string  `json:"title"`
	Slug          string  `json:"slug"`
	Department    string  `json:"department,omitempty"`
	City          string  `json:"city,omitempty"`
	State         string  `json:"state,omitempty"`
	Country       string  `json:"country,omitempty"`
	Remote        bool    `json:"remote"`
	Description   string  `json:"description"`
	CreatedAt     string  `json:"created_at"`
	CareersURL    string  `json:"careers_url"`
	SalaryMin     float64 `json:"salary_min,omitempty"`
	SalaryMax     float64 `json:"salary_max,omitempty"`
	SalaryCurrency string `json:"salary_currency,omitempty"`
}

func (s *Scraper) buildURL(slug string) string {
	if s.apiURL != "" {
		return s.apiURL
	}
	return fmt.Sprintf("https://%s.recruitee.com/api/offers", url.PathEscape(slug))
}

func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{"site": s.SiteName(), "results_wanted": input.ResultsWanted})

	seeds, src := ats.ResolveSeeds(input.SearchTerm, "SCRAPPY_RECRUITEE_SEEDS")
	if len(seeds) == 0 {
		return nil, fmt.Errorf("recruitee no seeds: set SCRAPPY_RECRUITEE_SEEDS or pass a company slug in --search (e.g. --search 'acme' resolves to acme.recruitee.com)")
	}
	util.Debug("recruitee_seeds", map[string]any{"seeds": seeds, "src": src})

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
		resp := new(recruiteeResponse)
		if err := ats.FetchJSON(ctx, s.client, u, resp); err != nil {
			util.Warn("recruitee_fetch_fail", map[string]any{"slug": slug, "err": err.Error()})
			continue
		}

		for _, offer := range resp.Offers {
			if len(out) >= wanted {
				break
			}
			title := strings.TrimSpace(offer.Title)
			if title == "" {
				continue
			}

			loc := model.Location{
				City:    strings.TrimSpace(offer.City),
				State:   strings.TrimSpace(offer.State),
				Country: strings.TrimSpace(offer.Country),
			}

			jobURL := ""
			if offer.CareersURL != "" && offer.Slug != "" {
				jobURL = fmt.Sprintf("%s/%s", strings.TrimRight(offer.CareersURL, "/"), offer.Slug)
			} else {
				jobURL = fmt.Sprintf("https://%s.recruitee.com/o/%s", slug, offer.Slug)
			}

			jp := model.JobPost{
				ID:          fmt.Sprintf("recruitee-%d", offer.ID),
				Title:       title,
				CompanyName: slug,
				JobURL:      jobURL,
				Location:    loc,
				IsRemote:    offer.Remote,
				Description: util.StripHTML(strings.TrimSpace(offer.Description)),
				Site:        string(model.SiteRecruitee),
				Department:  strings.TrimSpace(offer.Department),
			}

			if offer.SalaryMin != 0 || offer.SalaryMax != 0 {
				minAmt := offer.SalaryMin
				maxAmt := offer.SalaryMax
				currency := strings.TrimSpace(offer.SalaryCurrency)
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

			if offer.CreatedAt != "" {
				jp.DatePosted = util.ParseDatePosted(offer.CreatedAt)
			}

			out = append(out, jp)
		}
	}

	if !util.HasMeaningfulJobs(out) {
		return nil, fmt.Errorf("recruitee no parseable jobs")
	}
	return out, nil
}
