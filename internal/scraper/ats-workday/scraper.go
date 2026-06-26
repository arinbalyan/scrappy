package workday

import (
	"bytes"
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

const workdayPageSize = 20

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

func (s *Scraper) SiteName() model.Site { return model.SiteWorkday }

type wdJobListItem struct {
	Title         string `json:"title,omitempty"`
	ExternalPath  string `json:"externalPath,omitempty"`
	LocationsText string `json:"locationsText,omitempty"`
	PostedOn      string `json:"postedOn,omitempty"`
	Subtitles     []struct {
		Instances []struct {
			Text string `json:"text"`
		} `json:"instances"`
	} `json:"subtitles,omitempty"`
}

type wdSearchResponse struct {
	Total       int             `json:"total"`
	JobPostings []wdJobListItem `json:"jobPostings"`
}

type wdSearchPayload struct {
	AppliedFacets map[string]interface{} `json:"appliedFacets"`
	Limit         int                    `json:"limit"`
	Offset        int                    `json:"offset"`
	SearchText    string                 `json:"searchText"`
}

func parseWorkdaySlug(slug string) (company, wdNumber, site string) {
	parts := strings.SplitN(slug, ":", 3)
	company = parts[0]
	wdNumber = "5"
	site = "External"
	if len(parts) > 1 && parts[1] != "" {
		wdNumber = parts[1]
	}
	if len(parts) > 2 && parts[2] != "" {
		site = parts[2]
	}
	return
}

func buildAPIURL(company, wdNumber, site string) string {
	return fmt.Sprintf("https://%s.wd%s.myworkdayjobs.com/wday/cxs/%s/%s/jobs", company, wdNumber, company, site)
}

var jobIDRe = regexp.MustCompile(`/(\d+)(?:/|$)`)

func (s *Scraper) buildURL(company, wdNumber, site string) string {
	if s.apiURL != "" {
		return s.apiURL
	}
	return buildAPIURL(company, wdNumber, site)
}

func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{"site": s.SiteName(), "results_wanted": input.ResultsWanted})

	seeds, src := ats.ResolveSeeds(input.SearchTerm, "SCRAPPY_WORKDAY_SEEDS")
	if len(seeds) == 0 {
		return nil, fmt.Errorf("workday no seeds: set SCRAPPY_WORKDAY_SEEDS or pass a company slug in --search (format: company[:wdNumber[:site]], e.g. --search 'tesla' or --search 'tesla:5:Tesla')")
	}
	// If the search term was used as slug and it looks like a search phrase,
	// return early — Workday needs company subdomains, not a search string.
	if src == ats.SeedFromSearch && (strings.ContainsAny(seeds[0], " \"") || strings.Contains(seeds[0], "OR")) {
		return nil, fmt.Errorf("workday: no tenant slugs — got search term %q; set SCRAPPY_WORKDAY_SEEDS or pass --search 'tesla'", seeds[0])
	}
	util.Debug("workday_seeds", map[string]any{"seeds": seeds, "src": src})

	wanted := input.ResultsWanted
	if wanted <= 0 {
		wanted = 100
	}

	fetchFn := func(ctx context.Context, slug string) ([]model.JobPost, error) {
		company, wdNumber, site := parseWorkdaySlug(slug)
		u := s.buildURL(company, wdNumber, site)
		offset := 0

		var jobs []model.JobPost
		for {
			payload := wdSearchPayload{
				AppliedFacets: map[string]interface{}{},
				Limit:         workdayPageSize,
				Offset:        offset,
				SearchText:    "",
			}
			body, err := json.Marshal(payload)
			if err != nil {
				util.Warn("workday_marshal_fail", map[string]any{"company": company, "err": err.Error()})
				return jobs, nil
			}

			req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(body))
			if err != nil {
				util.Warn("workday_request_err", map[string]any{"company": company, "err": err.Error()})
				return jobs, nil
			}
			req.Header.Set("Accept", "application/json")
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("User-Agent", "Mozilla/5.0")

			resp, err := s.client.Do(req)
			if err != nil {
				util.Warn("workday_fetch_fail", map[string]any{"company": company, "offset": offset, "err": err.Error()})
				return jobs, nil
			}

			respBody, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
			resp.Body.Close()
			if err != nil {
				util.Warn("workday_read_fail", map[string]any{"company": company, "err": err.Error()})
				return jobs, nil
			}

			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				util.Warn("workday_status", map[string]any{"company": company, "offset": offset, "status": resp.StatusCode})
				return jobs, nil
			}

			var searchResp wdSearchResponse
			if err := json.Unmarshal(respBody, &searchResp); err != nil {
				util.Warn("workday_decode_fail", map[string]any{"company": company, "err": err.Error()})
				return jobs, nil
			}

			if len(searchResp.JobPostings) == 0 {
				break
			}

			for _, listing := range searchResp.JobPostings {
				jp := s.toJobPost(listing, company, wdNumber, site)
				if jp != nil {
					jobs = append(jobs, *jp)
				}
			}

			offset += len(searchResp.JobPostings)
			if len(searchResp.JobPostings) < workdayPageSize {
				break
			}
		}
		return jobs, nil
	}

	results := ats.ProcessSeeds(ctx, seeds, 3, wanted, fetchFn)
	if len(results) == 0 {
		return nil, fmt.Errorf("workday no parseable jobs")
	}
	return results, nil
}

func (s *Scraper) toJobPost(listing wdJobListItem, company, wdNumber, site string) *model.JobPost {
	title := strings.TrimSpace(listing.Title)
	if title == "" {
		return nil
	}

	externalPath := strings.TrimSpace(listing.ExternalPath)
	var jobURL string
	if externalPath != "" {
		if strings.HasPrefix(externalPath, "/") {
			jobURL = fmt.Sprintf("https://%s.wd%s.myworkdayjobs.com%s", company, wdNumber, externalPath)
		} else {
			jobURL = fmt.Sprintf("https://%s.wd%s.myworkdayjobs.com/%s", company, wdNumber, externalPath)
		}
	} else {
		jobURL = fmt.Sprintf("https://%s.wd%s.myworkdayjobs.com/en-US/%s", company, wdNumber, site)
	}

	loc := model.Location{}
	isRemote := false
	locStr := strings.TrimSpace(listing.LocationsText)
	if locStr != "" {
		loc.City = locStr
		if strings.Contains(strings.ToLower(locStr), "remote") {
			isRemote = true
		}
	}

	var dep string
	for _, sub := range listing.Subtitles {
		for _, inst := range sub.Instances {
			if strings.TrimSpace(inst.Text) != "" {
				dep = strings.TrimSpace(inst.Text)
				break
			}
		}
		if dep != "" {
			break
		}
	}

	// Extract job ID from externalPath
	atsID := ""
	if m := jobIDRe.FindStringSubmatch(externalPath); len(m) > 1 {
		atsID = m[1]
	}
	if atsID == "" {
		atsID = externalPath
	}

	jp := &model.JobPost{
		ID:          fmt.Sprintf("wd-%s-%s", company, atsID),
		Title:       title,
		CompanyName: company,
		JobURL:      jobURL,
		Location:    loc,
		IsRemote:    isRemote,
		Site:        string(model.SiteWorkday),
		Department:  dep,
	}

	if listing.PostedOn != "" {
		jp.DatePosted = util.ParseDatePosted(listing.PostedOn)
	}

	return jp
}
