package teamtailor

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

const teamtailorAPIURL = "https://career.teamtailor.com/widget/jobs"

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

func (s *Scraper) SiteName() model.Site { return model.SiteTeamTailor }

type ttJob struct {
	ID            string      `json:"id"`
	Type          string      `json:"type"`
	Attributes    *ttAttrs    `json:"attributes,omitempty"`
	Relationships *ttRel      `json:"relationships,omitempty"`
	Links         *struct {
		Self         string `json:"self,omitempty"`
		CareersiteURL string `json:"careersite-url,omitempty"`
	} `json:"links,omitempty"`
}

type ttAttrs struct {
	Title         string `json:"title"`
	Body          string `json:"body,omitempty"`
	EmploymentType string `json:"employment-type,omitempty"`
	ExternalURL   string `json:"external-url,omitempty"`
	ApplyURL      string `json:"apply-url,omitempty"`
	Remote        bool   `json:"remote"`
	City          string `json:"city,omitempty"`
	Region        string `json:"region,omitempty"`
	Country       string `json:"country,omitempty"`
	CreatedAt     string `json:"created-at,omitempty"`
}

type ttRel struct {
	Department *struct {
		Data *struct {
			ID string `json:"id"`
		} `json:"data"`
	} `json:"department,omitempty"`
}

type ttResponse struct {
	Data  []ttJob `json:"data"`
	Links *struct {
		Next string `json:"next,omitempty"`
	} `json:"links,omitempty"`
}

func (s *Scraper) buildURL(slug string) string {
	if s.apiURL != "" {
		return s.apiURL
	}
	return fmt.Sprintf("%s/%s", teamtailorAPIURL, url.PathEscape(slug))
}

func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{"site": s.SiteName(), "results_wanted": input.ResultsWanted})

	seeds, src := ats.ResolveSeeds(input.SearchTerm, "SCRAPPY_TEAMTAILOR_SEEDS")
	if len(seeds) == 0 {
		return nil, fmt.Errorf("teamtailor no seeds: set SCRAPPY_TEAMTAILOR_SEEDS or pass a company slug in --search")
	}
	util.Debug("teamtailor_seeds", map[string]any{"seeds": seeds, "src": src})

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
		resp := new(ttResponse)
		if err := ats.FetchJSON(ctx, s.client, u, resp); err != nil {
			util.Warn("teamtailor_fetch_fail", map[string]any{"slug": slug, "err": err.Error()})
			continue
		}

		for _, job := range resp.Data {
			if len(out) >= wanted {
				break
			}
			jp := s.toJobPost(job, slug)
			if jp != nil {
				out = append(out, *jp)
			}
		}
	}

	if !util.HasMeaningfulJobs(out) {
		return nil, fmt.Errorf("teamtailor no parseable jobs")
	}
	return out, nil
}

func (s *Scraper) toJobPost(job ttJob, slug string) *model.JobPost {
	if job.Attributes == nil {
		return nil
	}
	title := strings.TrimSpace(job.Attributes.Title)
	if title == "" {
		return nil
	}

	loc := model.Location{}
	if job.Attributes.City != "" {
		loc.City = strings.TrimSpace(job.Attributes.City)
		loc.State = strings.TrimSpace(job.Attributes.Region)
		loc.Country = strings.TrimSpace(job.Attributes.Country)
	}

	jobURL := ""
	if job.Links != nil && job.Links.CareersiteURL != "" {
		jobURL = job.Links.CareersiteURL
	} else if job.Attributes.ApplyURL != "" {
		jobURL = job.Attributes.ApplyURL
	} else if job.Attributes.ExternalURL != "" {
		jobURL = job.Attributes.ExternalURL
	} else {
		jobURL = fmt.Sprintf("https://career.teamtailor.com/%s/jobs/%s", slug, job.ID)
	}

	var dep string
	if job.Relationships != nil && job.Relationships.Department != nil && job.Relationships.Department.Data != nil {
		dep = job.Relationships.Department.Data.ID
	}

	jp := &model.JobPost{
		ID:          "teamtailor-" + job.ID,
		Title:       title,
		CompanyName: slug,
		JobURL:      jobURL,
		Location:    loc,
		IsRemote:    job.Attributes.Remote,
		Description: util.StripHTML(strings.TrimSpace(job.Attributes.Body)),
		Site:        string(model.SiteTeamTailor),
		Department:  dep,
	}

	if job.Attributes.CreatedAt != "" {
		jp.DatePosted = util.ParseDatePosted(job.Attributes.CreatedAt)
	}

	return jp
}
