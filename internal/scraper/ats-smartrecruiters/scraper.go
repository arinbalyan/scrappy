package smartrecruiters

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

const (
	smartrecruitersAPIURL   = "https://api.smartrecruiters.com/v1/companies"
	smartrecruitersPageSize = 100
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

func (s *Scraper) SiteName() model.Site { return model.SiteSmartRecruiters }

type srLocation struct {
	City    string `json:"city,omitempty"`
	Region  string `json:"region,omitempty"`
	Country string `json:"country,omitempty"`
	Remote  *bool  `json:"remote,omitempty"`
}

type srDepartment struct {
	ID    string `json:"id,omitempty"`
	Label string `json:"label,omitempty"`
}

type srJobAd struct {
	Sections *srSections `json:"sections,omitempty"`
}

type srSections struct {
	JobDescription        *srSectionContent `json:"jobDescription,omitempty"`
	Qualifications        *srSectionContent `json:"qualifications,omitempty"`
	AdditionalInformation *srSectionContent `json:"additionalInformation,omitempty"`
}

type srSectionContent struct {
	Title string `json:"title,omitempty"`
	Text  string `json:"text,omitempty"`
}

type srJob struct {
	ID               string         `json:"id,omitempty"`
	Name             string         `json:"name,omitempty"`
	ReleasedDate     string         `json:"releasedDate,omitempty"`
	Location         *srLocation    `json:"location,omitempty"`
	Department       *srDepartment  `json:"department,omitempty"`
	TypeOfEmployment *srDepartment  `json:"typeOfEmployment,omitempty"`
	Ref              string         `json:"ref,omitempty"`
	Company          *struct {
		Name       string `json:"name,omitempty"`
		Identifier string `json:"identifier,omitempty"`
	} `json:"company,omitempty"`
	JobAd *srJobAd `json:"jobAd,omitempty"`
}

type srResponse struct {
	Content []srJob `json:"content"`
}

func (s *Scraper) buildURL(slug string, offset int) string {
	if s.apiURL != "" {
		return s.apiURL
	}
	return fmt.Sprintf("%s/%s/postings?offset=%d&limit=%d", smartrecruitersAPIURL, url.PathEscape(slug), offset, smartrecruitersPageSize)
}

func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{"site": s.SiteName(), "results_wanted": input.ResultsWanted})

	seeds, src := ats.ResolveSeeds(input.SearchTerm, "SCRAPPY_SMARTRECRUITERS_SEEDS")
	if len(seeds) == 0 {
		return nil, fmt.Errorf("smartrecruiters no seeds: set SCRAPPY_SMARTRECRUITERS_SEEDS or pass a company slug in --search")
	}
	util.Debug("smartrecruiters_seeds", map[string]any{"seeds": seeds, "src": src})

	wanted := input.ResultsWanted
	if wanted <= 0 {
		wanted = 100
	}

	out := make([]model.JobPost, 0, wanted)
	for _, slug := range seeds {
		if len(out) >= wanted {
			break
		}

		offset := 0
		for {
			if len(out) >= wanted {
				break
			}

			u := s.buildURL(slug, offset)
			resp := new(srResponse)
			if err := ats.FetchJSON(ctx, s.client, u, resp); err != nil {
				util.Warn("smartrecruiters_fetch_fail", map[string]any{"slug": slug, "offset": offset, "err": err.Error()})
				break
			}

			if len(resp.Content) == 0 {
				break
			}

			for _, job := range resp.Content {
				if len(out) >= wanted {
					break
				}
				jp := s.toJobPost(job, slug)
				if jp != nil {
					out = append(out, *jp)
				}
			}

			offset += len(resp.Content)
			if len(resp.Content) < smartrecruitersPageSize {
				break
			}
		}
	}

	if !util.HasMeaningfulJobs(out) {
		return nil, fmt.Errorf("smartrecruiters no parseable jobs")
	}
	return out, nil
}

func (s *Scraper) toJobPost(job srJob, slug string) *model.JobPost {
	title := strings.TrimSpace(job.Name)
	if title == "" {
		return nil
	}

	loc := model.Location{}
	isRemote := false
	if job.Location != nil {
		loc.City = strings.TrimSpace(job.Location.City)
		loc.State = strings.TrimSpace(job.Location.Region)
		loc.Country = strings.TrimSpace(job.Location.Country)
		if job.Location.Remote != nil && *job.Location.Remote {
			isRemote = true
		}
	}

	jobURL := strings.TrimSpace(job.Ref)
	if jobURL == "" {
		jobURL = fmt.Sprintf("https://jobs.smartrecruiters.com/%s/%s", slug, job.ID)
	}

	company := slug
	if job.Company != nil && strings.TrimSpace(job.Company.Name) != "" {
		company = strings.TrimSpace(job.Company.Name)
	}

	var desc string
	if job.JobAd != nil && job.JobAd.Sections != nil {
		var parts []string
		if s := job.JobAd.Sections.JobDescription; s != nil && strings.TrimSpace(s.Text) != "" {
			parts = append(parts, s.Text)
		}
		if s := job.JobAd.Sections.Qualifications; s != nil && strings.TrimSpace(s.Text) != "" {
			parts = append(parts, s.Text)
		}
		if s := job.JobAd.Sections.AdditionalInformation; s != nil && strings.TrimSpace(s.Text) != "" {
			parts = append(parts, s.Text)
		}
		desc = util.StripHTML(strings.Join(parts, "\n"))
	}

	dep := ""
	if job.Department != nil {
		dep = strings.TrimSpace(job.Department.Label)
	}

	et := ""
	if job.TypeOfEmployment != nil {
		et = strings.TrimSpace(job.TypeOfEmployment.Label)
	}

	jp := &model.JobPost{
		ID:          "sr-" + job.ID,
		Title:       title,
		CompanyName: company,
		JobURL:      jobURL,
		Location:    loc,
		IsRemote:    isRemote,
		Description: desc,
		Site:        string(model.SiteSmartRecruiters),
		Department:  dep,
		JobType:     et,
	}

	if job.ReleasedDate != "" {
		jp.DatePosted = util.ParseDatePosted(job.ReleasedDate)
	}

	return jp
}
