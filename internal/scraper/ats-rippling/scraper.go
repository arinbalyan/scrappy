package rippling

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/scraper/ats"
	"github.com/arinbalyan/scrappy/internal/util"
)

const ripplingBaseURL = "https://ats.rippling.com"

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

func (s *Scraper) SiteName() model.Site { return model.SiteRippling }

// Types for __NEXT_DATA__ extraction
type nextData struct {
	Props *struct {
		PageProps *struct {
			DehydratedState *struct {
				Queries []struct {
					State *struct {
						Data *struct {
							Items []ripplingJob `json:"items"`
						} `json:"data"`
					} `json:"state"`
				} `json:"queries"`
			} `json:"dehydratedState"`
		} `json:"pageProps"`
	} `json:"props"`
}

type ripplingJob struct {
	UUID           string                   `json:"uuid"`
	ID             string                   `json:"id"`
	Title          string                   `json:"title"`
	Name           string                   `json:"name"`
	URL            string                   `json:"url"`
	Description    map[string]string        `json:"description,omitempty"`
	Locations      []ripplingLocation       `json:"locations,omitempty"`
	WorkLocations  []string                 `json:"workLocations,omitempty"`
	Department     map[string]string        `json:"department,omitempty"`
	EmploymentType map[string]string        `json:"employmentType,omitempty"`
	CreatedOn      string                   `json:"createdOn,omitempty"`
	CompanyName    string                   `json:"companyName,omitempty"`
	PayRange       []ripplingPayRange       `json:"payRangeDetails,omitempty"`
}

type ripplingLocation struct {
	City          string `json:"city"`
	State         string `json:"state"`
	Country       string `json:"country"`
	WorkplaceType string `json:"workplaceType"`
}

type ripplingPayRange struct {
	MinValue float64 `json:"min_value"`
	MaxValue float64 `json:"max_value"`
	Currency string  `json:"currency"`
	Interval string  `json:"interval"`
}

var nextDataRe = regexp.MustCompile(`<script\s+id="__NEXT_DATA__"\s+type="application/json"[^>]*>([\s\S]*?)</script>`)

func (s *Scraper) buildURL(slug string) string {
	if s.apiURL != "" {
		return s.apiURL
	}
	return fmt.Sprintf("%s/%s/jobs", ripplingBaseURL, slug)
}

func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{"site": s.SiteName(), "results_wanted": input.ResultsWanted})

	seeds, src := ats.ResolveSeeds(input.SearchTerm, "SCRAPPY_RIPPLING_SEEDS")
	if len(seeds) == 0 {
		return nil, fmt.Errorf("rippling no seeds: set SCRAPPY_RIPPLING_SEEDS or pass a company slug in --search")
	}
	util.Debug("rippling_seeds", map[string]any{"seeds": seeds, "src": src})

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

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			util.Warn("rippling_request_err", map[string]any{"slug": slug, "err": err.Error()})
			continue
		}
		req.Header.Set("Accept", "text/html,application/xhtml+xml")
		req.Header.Set("User-Agent", "Mozilla/5.0")

		resp, err := s.client.Do(req)
		if err != nil {
			util.Warn("rippling_fetch_fail", map[string]any{"slug": slug, "err": err.Error()})
			continue
		}

		body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
		resp.Body.Close()
		if err != nil {
			util.Warn("rippling_read_fail", map[string]any{"slug": slug, "err": err.Error()})
			continue
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			util.Warn("rippling_status", map[string]any{"slug": slug, "status": resp.StatusCode})
			continue
		}

		jobs, err := s.extractJobs(body)
		if err != nil {
			util.Warn("rippling_extract_fail", map[string]any{"slug": slug, "err": err.Error()})
			continue
		}

		for _, job := range jobs {
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
		return nil, fmt.Errorf("rippling no parseable jobs")
	}
	return out, nil
}

func (s *Scraper) extractJobs(body []byte) ([]ripplingJob, error) {
	match := nextDataRe.FindSubmatch(body)
	if len(match) < 2 {
		return nil, fmt.Errorf("__NEXT_DATA__ not found")
	}

	var nd nextData
	if err := json.Unmarshal(match[1], &nd); err != nil {
		return nil, fmt.Errorf("parse __NEXT_DATA__: %w", err)
	}

	if nd.Props != nil && nd.Props.PageProps != nil && nd.Props.PageProps.DehydratedState != nil {
		for _, q := range nd.Props.PageProps.DehydratedState.Queries {
			if q.State != nil && q.State.Data != nil && len(q.State.Data.Items) > 0 {
				return q.State.Data.Items, nil
			}
		}
	}

	return nil, fmt.Errorf("no job items in __NEXT_DATA__")
}

func (s *Scraper) toJobPost(job ripplingJob, slug string) *model.JobPost {
	title := strings.TrimSpace(job.Title)
	if title == "" {
		title = strings.TrimSpace(job.Name)
	}
	if title == "" {
		return nil
	}

	loc := model.Location{}
	isRemote := false
	if len(job.Locations) > 0 {
		l := job.Locations[0]
		loc.City = strings.TrimSpace(l.City)
		loc.State = strings.TrimSpace(l.State)
		loc.Country = strings.TrimSpace(l.Country)
		if strings.ToLower(l.WorkplaceType) == "remote" {
			isRemote = true
		}
	}
	if !isRemote {
		for _, wl := range job.WorkLocations {
			if strings.Contains(strings.ToLower(wl), "remote") {
				isRemote = true
				break
			}
		}
	}

	var desc string
	if job.Description != nil {
		var parts []string
		if job.Description["role"] != "" {
			parts = append(parts, job.Description["role"])
		}
		if job.Description["company"] != "" {
			parts = append(parts, job.Description["company"])
		}
		desc = strings.Join(parts, "\n\n")
	}

	jobURL := strings.TrimSpace(job.URL)
	if jobURL == "" {
		jobURL = fmt.Sprintf("%s/%s/jobs/%s", ripplingBaseURL, slug, job.UUID)
	}

	company := strings.TrimSpace(job.CompanyName)
	if company == "" {
		company = slug
	}

	var dep string
	if job.Department != nil {
		dep = job.Department["name"]
	}

	var empType string
	if job.EmploymentType != nil {
		empType = job.EmploymentType["label"]
	}

	id := job.UUID
	if id == "" {
		id = job.ID
	}

	jp := &model.JobPost{
		ID:          "rippling-" + id,
		Title:       title,
		CompanyName: company,
		JobURL:      jobURL,
		Location:    loc,
		IsRemote:    isRemote,
		Description: desc,
		Site:        string(model.SiteRippling),
		Department:  dep,
		JobType:     empType,
	}

	if job.CreatedOn != "" {
		jp.DatePosted = util.ParseDatePosted(job.CreatedOn)
	}

	if len(job.PayRange) > 0 && (job.PayRange[0].MinValue != 0 || job.PayRange[0].MaxValue != 0) {
		pr := job.PayRange[0]
		interval := model.IntervalYearly
		switch strings.ToLower(pr.Interval) {
		case "yearly", "annual":
			interval = model.IntervalYearly
		case "monthly":
			interval = model.IntervalMonthly
		case "weekly":
			interval = model.IntervalWeekly
		case "hourly":
			interval = model.IntervalHourly
		}
		minAmt := pr.MinValue
		maxAmt := pr.MaxValue
		currency := pr.Currency
		if currency == "" {
			currency = "USD"
		}
		jp.Compensation = &model.Compensation{
			Interval:  interval,
			MinAmount: &minAmt,
			MaxAmount: &maxAmt,
			Currency:  currency,
		}
	}

	return jp
}
