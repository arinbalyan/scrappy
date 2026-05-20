package ukvisajobs

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/util"
)

const defaultAPI = "https://www.ukvisajobs.com/"

var (
	reUKVTitle   = regexp.MustCompile(`<h5 style="cursor:pointer">([^<]+)</h5>`)
	reUKVCompany = regexp.MustCompile(`<div class="company company_1 tooltip-wrapper"><a [^>]*>([^<]+)<div class="info_icon">`)
	reUKVLoc     = regexp.MustCompile(`<li><img src="images/new/location\.svg" alt=""/>\s*<span>([^<]+)</span></li>`)
	reUKVDeg     = regexp.MustCompile(`<li><img src="images/new/teacher\.svg" alt=""/>\s*<span>([^<]+)</span></li>`)
	reUKVSalary  = regexp.MustCompile(`<li><img class="sm_icon_pound" src="/images/pound1\.png" alt=""/>\s*<span>([^<]+)</span></li>`)
)

type Scraper struct {
	client *http.Client
	apiURL string
}

func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Retries: 3, Timeout: 18 * time.Second})
	}
	return &Scraper{client: client, apiURL: defaultAPI}
}
func NewWithAPIURL(client *http.Client, endpoint string) *Scraper {
	s := New(client)
	if strings.TrimSpace(endpoint) != "" {
		s.apiURL = endpoint
	}
	return s
}
func (s *Scraper) SiteName() model.Site { return model.SiteUKVisaJobs }

func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{"site": s.SiteName(), "results_wanted": input.ResultsWanted, "search_term": input.SearchTerm, "location": input.Location})

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, s.apiURL, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ukvisajobs request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("ukvisajobs status %d", resp.StatusCode)
	}
	body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("ukvisajobs read: %w", err)
	}

	jobs := parseUKVisaJobsAPI(body)
	if !util.HasMeaningfulJobs(jobs) {
		jobs = parseUKVisaJobsHTML(string(body))
	}
	jobs = limitUKVisaJobs(jobs, input.ResultsWanted)
	if !util.HasMeaningfulJobs(jobs) {
		return nil, nil
	}
	util.Debug("scraper_done", map[string]any{"site": s.SiteName(), "jobs": len(jobs)})
	return jobs, nil
}

func parseUKVisaJobsAPI(body []byte) []model.JobPost {
	var parsed []struct{ ID, Title, Company, Location, URL, Description, PostedAt string }
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil
	}
	jobs := make([]model.JobPost, 0, len(parsed))
	for _, r := range parsed {
		title := strings.TrimSpace(r.Title)
		company := strings.TrimSpace(r.Company)
		if title == "" || company == "" {
			continue
		}
		var posted *time.Time
		if t, err := time.Parse(time.RFC3339, r.PostedAt); err == nil {
			posted = &t
		}
		jobs = append(jobs, model.JobPost{ID: "ukv-" + strings.TrimSpace(r.ID), Title: title, CompanyName: company, Location: model.Location{City: strings.TrimSpace(r.Location)}, JobURL: strings.TrimSpace(r.URL), Description: strings.TrimSpace(r.Description), DatePosted: posted})
	}
	return jobs
}

func parseUKVisaJobsHTML(raw string) []model.JobPost {
	titles := reUKVTitle.FindAllStringSubmatch(raw, -1)
	companies := reUKVCompany.FindAllStringSubmatch(raw, -1)
	locs := reUKVLoc.FindAllStringSubmatch(raw, -1)
	degs := reUKVDeg.FindAllStringSubmatch(raw, -1)
	salaries := reUKVSalary.FindAllStringSubmatch(raw, -1)

	limit := len(titles)
	if len(companies) < limit {
		limit = len(companies)
	}
	jobs := make([]model.JobPost, 0, limit)
	for i := 0; i < limit; i++ {
		title := strings.TrimSpace(titles[i][1])
		company := strings.TrimSpace(companies[i][1])
		if title == "" || company == "" {
			continue
		}
		loc := ""
		if i < len(locs) {
			loc = strings.TrimSpace(locs[i][1])
		}
		descParts := make([]string, 0, 2)
		if i < len(degs) {
			descParts = append(descParts, "Degree: "+strings.TrimSpace(degs[i][1]))
		}
		if i < len(salaries) {
			descParts = append(descParts, "Salary: "+strings.TrimSpace(salaries[i][1]))
		}
		id := fmt.Sprintf("ukv-%d", i+1)
		jobs = append(jobs, model.JobPost{ID: id, Title: title, CompanyName: company, Location: model.Location{City: loc}, JobURL: "https://my.ukvisajobs.com/open-jobs/1", Description: strings.Join(descParts, " | ")})
	}
	return jobs
}

func limitUKVisaJobs(in []model.JobPost, wanted int) []model.JobPost {
	if wanted <= 0 || wanted > len(in) {
		return in
	}
	return in[:wanted]
}
