package ukg

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

const ukgAPIURL = "https://recruiting.ultipro.com"

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

func (s *Scraper) SiteName() model.Site { return model.SiteUKG }

type ukgLocation struct {
	City            string `json:"city,omitempty"`
	State           string `json:"state,omitempty"`
	Country         string `json:"country,omitempty"`
	FormattedAddress string `json:"formattedAddress,omitempty"`
}

type ukgJob struct {
	ID                string        `json:"id,omitempty"`
	Title             string        `json:"title,omitempty"`
	Description       string        `json:"description,omitempty"`
	ShortDescription  string        `json:"shortDescription,omitempty"`
	Department        string        `json:"department,omitempty"`
	Category          string        `json:"category,omitempty"`
	Location          *ukgLocation  `json:"location,omitempty"`
	Locations         []ukgLocation `json:"locations,omitempty"`
	JobType           string        `json:"jobType,omitempty"`
	PostedDate        string        `json:"postedDate,omitempty"`
	ApplyURL          string        `json:"applyUrl,omitempty"`
	CompanyName       string        `json:"companyName,omitempty"`
	RequisitionNumber string        `json:"requisitionNumber,omitempty"`
}

type ukgResponse struct {
	Opportunities []ukgJob `json:"opportunities"`
}

func (s *Scraper) buildURL(slug string) string {
	if s.apiURL != "" {
		return s.apiURL
	}
	return fmt.Sprintf("%s/%s/OpportunitySearch", ukgAPIURL, url.PathEscape(slug))
}

func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{"site": s.SiteName(), "results_wanted": input.ResultsWanted})

	seeds, src := ats.ResolveSeeds(input.SearchTerm, "SCRAPPY_UKG_SEEDS")
	if len(seeds) == 0 {
		return nil, fmt.Errorf("ukg no seeds: set SCRAPPY_UKG_SEEDS or pass a company slug in --search")
	}
	util.Debug("ukg_seeds", map[string]any{"seeds": seeds, "src": src})

	wanted := input.ResultsWanted
	if wanted <= 0 {
		wanted = 100
	}

	fetchFn := func(ctx context.Context, slug string) ([]model.JobPost, error) {
		u := s.buildURL(slug)
		resp := new(ukgResponse)
		if err := ats.FetchJSON(ctx, s.client, u, resp); err != nil {
			util.Warn("ukg_fetch_fail", map[string]any{"slug": slug, "err": err.Error()})
			return nil, err
		}

		var jobs []model.JobPost
		for _, job := range resp.Opportunities {
			title := strings.TrimSpace(job.Title)
			if title == "" {
				continue
			}

			loc := s.extractLocation(job)
			isRemote := s.isRemote(job)

			jobURL := strings.TrimSpace(job.ApplyURL)
			if jobURL == "" {
				jobURL = fmt.Sprintf("%s/%s/OpportunityDetail?opportunityId=%s", ukgAPIURL, slug, job.ID)
			}

			company := strings.TrimSpace(job.CompanyName)
			if company == "" {
				company = slug
			}

			desc := strings.TrimSpace(job.Description)
			if desc == "" {
				desc = strings.TrimSpace(job.ShortDescription)
			}

			dep := strings.TrimSpace(job.Department)
			if dep == "" {
				dep = strings.TrimSpace(job.Category)
			}

			id := job.ID
			if id == "" {
				id = job.RequisitionNumber
			}

			jp := model.JobPost{
				ID:          fmt.Sprintf("ukg-%s", id),
				Title:       title,
				CompanyName: company,
				JobURL:      jobURL,
				Location:    loc,
				IsRemote:    isRemote,
				Description: util.StripHTML(desc),
				Site:        string(model.SiteUKG),
				Department:  dep,
				JobType:     strings.TrimSpace(job.JobType),
			}
			if job.PostedDate != "" {
				jp.DatePosted = util.ParseDatePosted(job.PostedDate)
			}
			jobs = append(jobs, jp)
		}
		return jobs, nil
	}

	results := ats.ProcessSeeds(ctx, seeds, 3, wanted, fetchFn)
	if len(results) == 0 {
		return nil, fmt.Errorf("ukg no parseable jobs")
	}
	return results, nil
}

func (s *Scraper) extractLocation(job ukgJob) model.Location {
	loc := model.Location{}
	if len(job.Locations) > 0 {
		l := job.Locations[0]
		loc.City = strings.TrimSpace(l.City)
		loc.State = strings.TrimSpace(l.State)
		loc.Country = strings.TrimSpace(l.Country)
	} else if job.Location != nil {
		loc.City = strings.TrimSpace(job.Location.City)
		loc.State = strings.TrimSpace(job.Location.State)
		loc.Country = strings.TrimSpace(job.Location.Country)
	}
	return loc
}

func (s *Scraper) isRemote(job ukgJob) bool {
	candidates := []string{}
	if len(job.Locations) > 0 {
		l := job.Locations[0]
		candidates = append(candidates, l.FormattedAddress, l.City)
	} else if job.Location != nil {
		candidates = append(candidates, job.Location.FormattedAddress, job.Location.City)
	}
	for _, c := range candidates {
		if strings.Contains(strings.ToLower(c), "remote") {
			return true
		}
	}
	return false
}
