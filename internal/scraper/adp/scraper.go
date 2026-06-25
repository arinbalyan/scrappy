package adp

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/scraper/ats"
	"github.com/arinbalyan/scrappy/internal/util"
)

const adpAPIURL = "https://workforcenow.adp.com/mascsr/default/careercenter/public/events/staffing/v1/job-requisitions"

// Scraper fetches jobs from ADP Workforce Now.
type Scraper struct {
	client *http.Client
	apiURL string
}

// New creates a new ADP scraper.
func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Timeout: 20 * time.Second})
	}
	return &Scraper{client: client, apiURL: adpAPIURL}
}

// NewWithAPIURL creates a new ADP scraper with a custom API URL (used in tests).
func NewWithAPIURL(client *http.Client, apiURL string) *Scraper {
	s := New(client)
	if strings.TrimSpace(apiURL) != "" {
		s.apiURL = apiURL
	}
	return s
}

func (s *Scraper) SiteName() model.Site { return model.SiteADP }

// --- API response types ---

type adpResponse struct {
	JobRequisitions []adpJob `json:"jobRequisitions"`
}

type adpLocation struct {
	City            string `json:"city"`
	StateProvince   string `json:"stateProvince"`
	Country         string `json:"country"`
	FormattedAddress string `json:"formattedAddress"`
}

type adpCompensation struct {
	MinPay   *float64 `json:"minPay"`
	MaxPay   *float64 `json:"maxPay"`
	Currency string   `json:"currency"`
	Frequency string  `json:"frequency"`
}

type adpJob struct {
	JobRequisitionID string         `json:"jobRequisitionId"`
	RequisitionNumber string        `json:"requisitionNumber"`
	JobTitle         string         `json:"jobTitle"`
	JobDescription   string         `json:"jobDescription"`
	ShortDescription string         `json:"shortDescription"`
	DepartmentName   string         `json:"departmentName"`
	Location         *adpLocation   `json:"location"`
	Locations        []adpLocation  `json:"locations"`
	WorkerTypeCode   string         `json:"workerTypeCode"`
	EmploymentType   string         `json:"employmentType"`
	ExternalURL      string         `json:"externalUrl"`
	PostedDate       string         `json:"postedDate"`
	CompanyName      string         `json:"companyName"`
	Compensation     *adpCompensation `json:"compensation"`
}

// Scrape fetches jobs from ADP for the given company seeds.
func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{"site": s.SiteName(), "results_wanted": input.ResultsWanted})

	seeds, src := ats.ResolveSeeds(input.SearchTerm, "SCRAPPY_ADP_SEEDS")
	if len(seeds) == 0 {
		return nil, fmt.Errorf("adp no seeds: set SCRAPPY_ADP_SEEDS or pass a company slug in --search")
	}
	util.Debug("adp_seeds", map[string]any{"seeds": seeds, "src": src})

	wanted := input.ResultsWanted
	if wanted <= 0 {
		wanted = 25
	}

	out := make([]model.JobPost, 0, wanted)
	seen := make(map[string]bool)
	var mu sync.Mutex

	fetchFn := func(ctx context.Context, slug string) ([]model.JobPost, error) {
		u := s.apiURL + "?cid=" + url.QueryEscape(slug)
		var resp adpResponse
		if err := ats.FetchJSON(ctx, s.client, u, &resp); err != nil {
			util.Debug("adp_fetch_skip", map[string]any{"slug": slug, "err": err.Error()})
			return nil, err
		}
		var jobs []model.JobPost
		for _, job := range resp.JobRequisitions {
			title := strings.TrimSpace(job.JobTitle)
			if title == "" {
				continue
			}
			id := ats.BuildID("adp", slug, job.JobRequisitionID)
			mu.Lock()
			if seen[id] {
				mu.Unlock()
				continue
			}
			seen[id] = true
			mu.Unlock()

			desc := strings.TrimSpace(job.JobDescription)
			if desc == "" {
				desc = strings.TrimSpace(job.ShortDescription)
			}

			var loc *adpLocation
			if len(job.Locations) > 0 {
				loc = &job.Locations[0]
			} else {
				loc = job.Location
			}

			l := model.Location{}
			isRemote := false
			if loc != nil {
				l.City = strings.TrimSpace(loc.City)
				l.State = strings.TrimSpace(loc.StateProvince)
				l.Country = strings.TrimSpace(loc.Country)
				srcStr := strings.ToLower(loc.FormattedAddress + " " + loc.City)
				isRemote = strings.Contains(srcStr, "remote")
			}

			// Job URL
			jobURL := strings.TrimSpace(job.ExternalURL)
			if jobURL == "" {
				jobURL = fmt.Sprintf(
					"https://workforcenow.adp.com/mascsr/default/mdf/recruitment/recruitment.html?cid=%s&jobId=%s",
					url.QueryEscape(slug),
					job.JobRequisitionID,
				)
			}

			companyName := strings.TrimSpace(job.CompanyName)
			if companyName == "" {
				companyName = slug
			}

			jp := model.JobPost{
				ID:           id,
				Title:        title,
				CompanyName:  companyName,
				JobURL:       jobURL,
				Location:     l,
				IsRemote:     isRemote,
				Description:  util.StripHTML(desc),
				Site:         string(s.SiteName()),
				Department:   strings.TrimSpace(job.DepartmentName),
				JobType:      normalizeEmploymentType(job.EmploymentType, job.WorkerTypeCode),
			}

			if dp := strings.TrimSpace(job.PostedDate); dp != "" {
				jp.DatePosted = util.ParseDatePosted(dp)
			}

			out = append(out, jp)
		}
		return jobs, nil
	}

	// Process seeds concurrently (3 workers) to reduce total scrape time.
	results := ats.ProcessSeeds(ctx, seeds, 3, wanted, fetchFn)
	if len(results) == 0 {
		return nil, fmt.Errorf("adp no parseable jobs")
	}
	return results, nil
}

func normalizeEmploymentType(employmentType, workerType string) string {
	v := strings.ToLower(strings.TrimSpace(employmentType))
	if v != "" {
		switch {
		case strings.Contains(v, "full"):
			return "fulltime"
		case strings.Contains(v, "part"):
			return "parttime"
		case strings.Contains(v, "contract"), strings.Contains(v, "temp"):
			return "contract"
		case strings.Contains(v, "intern"):
			return "internship"
		}
	}
	v = strings.ToLower(strings.TrimSpace(workerType))
	switch v {
	case "f", "ft", "fulltime", "regular":
		return "fulltime"
	case "p", "pt", "parttime":
		return "parttime"
	case "c", "contractor", "temp":
		return "contract"
	case "i", "intern":
		return "internship"
	}
	return ""
}
