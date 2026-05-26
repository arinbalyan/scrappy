package recruiterflow

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/scraper/ats"
	"github.com/arinbalyan/scrappy/internal/util"
)

const recruiterflowAPIBase = "https://api.recruiterflow.com/api/external/job/list"

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

func (s *Scraper) SiteName() model.Site { return model.SiteRecruiterFlow }

type rfAPIResponse struct {
	Data       []rfJob `json:"data"`
	TotalItems int     `json:"total_items"`
}

type rfJob struct {
	ID          int    `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Location    string `json:"location,omitempty"`
	Status      string `json:"status,omitempty"`
	CreatedAt   string `json:"created_at,omitempty"`
}

func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{"site": s.SiteName(), "results_wanted": input.ResultsWanted})

	seeds, src := ats.ResolveSeeds(input.SearchTerm, "SCRAPPY_RECRUITERFLOW_SEEDS")
	if len(seeds) == 0 {
		return nil, fmt.Errorf("recruiterflow no seeds: set SCRAPPY_RECRUITERFLOW_SEEDS or pass a company slug in --search")
	}
	util.Debug("recruiterflow_seeds", map[string]any{"seeds": seeds, "src": src})

	wanted := input.ResultsWanted
	if wanted <= 0 {
		wanted = 100
	}

	out := make([]model.JobPost, 0, wanted)
	for _, slug := range seeds {
		if len(out) >= wanted {
			break
		}

		u := s.buildURL()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			util.Warn("recruiterflow_request_err", map[string]any{"slug": slug, "err": err.Error()})
			continue
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "Mozilla/5.0")
		req.Header.Set("RF-Api-Key", slug)

		resp, err := s.client.Do(req)
		if err != nil {
			util.Warn("recruiterflow_fetch_fail", map[string]any{"slug": slug, "err": err.Error()})
			continue
		}

		body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
		resp.Body.Close()
		if err != nil {
			util.Warn("recruiterflow_read_fail", map[string]any{"slug": slug, "err": err.Error()})
			continue
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			util.Warn("recruiterflow_status", map[string]any{"slug": slug, "status": resp.StatusCode})
			continue
		}

		var apiResp rfAPIResponse
		if err := json.Unmarshal(body, &apiResp); err != nil {
			util.Warn("recruiterflow_decode_fail", map[string]any{"slug": slug, "err": err.Error()})
			continue
		}

		for _, job := range apiResp.Data {
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
		return nil, fmt.Errorf("recruiterflow no parseable jobs")
	}
	return out, nil
}

func (s *Scraper) buildURL() string {
	if s.apiURL != "" {
		return s.apiURL
	}
	return recruiterflowAPIBase
}

func (s *Scraper) toJobPost(job rfJob, slug string) *model.JobPost {
	title := strings.TrimSpace(job.Title)
	if title == "" {
		return nil
	}

	loc := model.Location{}
	isRemote := false
	locStr := strings.TrimSpace(job.Location)
	if locStr != "" {
		loc.City = locStr
		if strings.Contains(strings.ToLower(locStr), "remote") {
			isRemote = true
		}
	}

	jobURL := fmt.Sprintf("https://recruiterflow.com/jobs/%s/%d", url.PathEscape(slug), job.ID)

	jp := &model.JobPost{
		ID:          fmt.Sprintf("recruiterflow-%d", job.ID),
		Title:       title,
		CompanyName: slug,
		JobURL:      jobURL,
		Location:    loc,
		IsRemote:    isRemote,
		Description: util.StripHTML(strings.TrimSpace(job.Description)),
		Site:        string(model.SiteRecruiterFlow),
	}
	if job.CreatedAt != "" {
		jp.DatePosted = util.ParseDatePosted(job.CreatedAt)
	}
	return jp
}
