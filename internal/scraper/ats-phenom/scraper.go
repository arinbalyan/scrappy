package phenom

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

const phenomPageSize = 25

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

func (s *Scraper) SiteName() model.Site { return model.SitePhenom }

type phenomJob struct {
	ID             int            `json:"id"`
	ReqID          string         `json:"reqId,omitempty"`
	Title          string         `json:"title"`
	Description    string         `json:"description,omitempty"`
	ShortDesc      string         `json:"shortDescription,omitempty"`
	Location       *phenomLocation `json:"location,omitempty"`
	LocationText   string         `json:"locationText,omitempty"`
	Department     string         `json:"department,omitempty"`
	Category       string         `json:"category,omitempty"`
	Type           string         `json:"type,omitempty"`
	EmploymentType string         `json:"employmentType,omitempty"`
	PostedDate     jsonNumber     `json:"postedDate,omitempty"`
	URL            string         `json:"url,omitempty"`
	ApplyURL       string         `json:"applyUrl,omitempty"`
	CompanyName    string         `json:"companyName,omitempty"`
	IsRemote       *bool          `json:"isRemote,omitempty"`
	WorkplaceType  string         `json:"workplaceType,omitempty"`
}

type jsonNumber float64
func (j *jsonNumber) UnmarshalJSON(b []byte) error {
	if len(b) == 0 || string(b) == "null" {
		return nil
	}
	var f float64
	if err := json.Unmarshal(b, &f); err != nil {
		return nil // ignore parse errors
	}
	*j = jsonNumber(f)
	return nil
}

type phenomLocation struct {
	City    string `json:"city"`
	State   string `json:"state"`
	Country string `json:"country"`
}

type phenomResponse struct {
	Jobs []phenomJob `json:"jobs"`
}

func (s *Scraper) buildURL(slug string) string {
	if s.apiURL != "" {
		return s.apiURL
	}
	return fmt.Sprintf("https://jobs.%s.com/api/jobs", slug)
}

func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{"site": s.SiteName(), "results_wanted": input.ResultsWanted})

	seeds, src := ats.ResolveSeeds(input.SearchTerm, "SCRAPPY_PHENOM_SEEDS")
	if len(seeds) == 0 {
		return nil, fmt.Errorf("phenom no seeds: set SCRAPPY_PHENOM_SEEDS or pass a company slug in --search")
	}
	util.Debug("phenom_seeds", map[string]any{"seeds": seeds, "src": src})

	wanted := input.ResultsWanted
	if wanted <= 0 {
		wanted = 100
	}

	out := make([]model.JobPost, 0, wanted)
	for _, slug := range seeds {
		if len(out) >= wanted {
			break
		}

		resp := new(phenomResponse)
		u := fmt.Sprintf(s.buildURL(slug)+"?offset=0&limit=%d", phenomPageSize)
		if s.apiURL != "" {
			u = s.apiURL
		}
		if err := ats.FetchJSON(ctx, s.client, u, resp); err != nil {
			util.Warn("phenom_fetch_fail", map[string]any{"slug": slug, "err": err.Error()})
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
			if job.Location != nil {
				loc.City = strings.TrimSpace(job.Location.City)
				loc.State = strings.TrimSpace(job.Location.State)
				loc.Country = strings.TrimSpace(job.Location.Country)
			} else if job.LocationText != "" {
				loc.City = strings.TrimSpace(job.LocationText)
			}

			isRemote := s.detectRemote(job, loc)

			jobURL := job.URL
			if jobURL == "" {
				jobURL = job.ApplyURL
			}
			if jobURL == "" {
				jobURL = fmt.Sprintf("https://jobs.%s.com/job/%d", slug, job.ID)
			}

			desc := strings.TrimSpace(job.Description)
			if desc == "" {
				desc = strings.TrimSpace(job.ShortDesc)
			}

			dep := strings.TrimSpace(job.Department)
			if dep == "" {
				dep = strings.TrimSpace(job.Category)
			}

			et := strings.TrimSpace(job.EmploymentType)
			if et == "" {
				et = strings.TrimSpace(job.Type)
			}

			id := fmt.Sprintf("phenom-%d", job.ID)
			if job.ReqID != "" {
				id = "phenom-" + job.ReqID
			}

			jp := model.JobPost{
				ID:          id,
				Title:       title,
				CompanyName: nonEmpty(job.CompanyName, slug),
				JobURL:      jobURL,
				Location:    loc,
				IsRemote:    isRemote,
				Description: desc,
				Site:        string(model.SitePhenom),
				Department:  dep,
				JobType:     et,
			}
			if jp.DatePosted = parseDate(float64(job.PostedDate)); jp.DatePosted != nil {
				// ok
			}
			out = append(out, jp)
		}
	}

	if !util.HasMeaningfulJobs(out) {
		return nil, fmt.Errorf("phenom no parseable jobs")
	}
	return out, nil
}

func (s *Scraper) detectRemote(job phenomJob, loc model.Location) bool {
	if job.IsRemote != nil && *job.IsRemote {
		return true
	}
	fields := []string{job.WorkplaceType, job.LocationText, job.Type, job.Title}
	fields = append(fields, loc.City, loc.State, loc.Country)
	for _, f := range fields {
		if strings.Contains(strings.ToLower(f), "remote") {
			return true
		}
	}
	return false
}

func parseDate(v float64) *time.Time {
	if v == 0 {
		return nil
	}
	// Could be epoch ms (13-digit) or seconds (10-digit)
	var sec int64
	if v > 1e12 {
		sec = int64(v) / 1000
	} else {
		sec = int64(v)
	}
	t := time.Unix(sec, 0)
	return &t
}

func nonEmpty(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}
