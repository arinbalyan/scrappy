package gradcracker

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

const defaultAPI = "https://www.gradcracker.com/search/all-disciplines/engineering-work-placements-internships"

var (
	reJobAnchor = regexp.MustCompile(`href="(https://www\.gradcracker\.com/hub/[^"]+/work-placement-internship/[^"]+)"`)
	reJobTitle  = regexp.MustCompile(`aria-label="Apply for the ([^"]+?) opportunity with [^"]+"`)
	reEmployer  = regexp.MustCompile(`aria-label="Apply for the [^"]+? opportunity with ([^"]+)"`)
	reDeadline  = regexp.MustCompile(`Deadline:\s*([^<\n]+)`)
	reLocation  = regexp.MustCompile(`<dt>Location</dt>\s*<dd>([^<]+)</dd>`)
	reSalary    = regexp.MustCompile(`<dt>Salary</dt>\s*<dd>([^<]+)</dd>`)
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
func (s *Scraper) SiteName() model.Site { return model.SiteGradcracker }

func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{"site": s.SiteName(), "results_wanted": input.ResultsWanted, "search_term": input.SearchTerm, "location": input.Location})

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, s.apiURL, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gradcracker request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("gradcracker status %d", resp.StatusCode)
	}
	body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("gradcracker read: %w", err)
	}

	jobs := parseGradcrackerAPI(body)
	if !util.HasMeaningfulJobs(jobs) {
		jobs = parseGradcrackerHTML(string(body))
	}
	jobs = limitGradcrackerJobs(jobs, input.ResultsWanted)
	if !util.HasMeaningfulJobs(jobs) {
		return nil, nil
	}
	util.Debug("scraper_done", map[string]any{"site": s.SiteName(), "jobs": len(jobs)})
	return jobs, nil
}

func parseGradcrackerAPI(body []byte) []model.JobPost {
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
		jobs = append(jobs, model.JobPost{ID: "gc-" + strings.TrimSpace(r.ID), Title: title, CompanyName: company, Location: model.Location{City: strings.TrimSpace(r.Location)}, JobURL: strings.TrimSpace(r.URL), Description: strings.TrimSpace(r.Description), DatePosted: posted})
	}
	return jobs
}

func parseGradcrackerHTML(raw string) []model.JobPost {
	urls := reJobAnchor.FindAllStringSubmatch(raw, -1)
	titles := reJobTitle.FindAllStringSubmatch(raw, -1)
	employers := reEmployer.FindAllStringSubmatch(raw, -1)
	deadlines := reDeadline.FindAllStringSubmatch(raw, -1)
	locations := reLocation.FindAllStringSubmatch(raw, -1)
	salaries := reSalary.FindAllStringSubmatch(raw, -1)

	seen := make(map[string]struct{}, len(urls))
	jobs := make([]model.JobPost, 0, len(urls))
	for i, m := range urls {
		jobURL := strings.TrimSpace(m[1])
		if jobURL == "" {
			continue
		}
		if _, ok := seen[jobURL]; ok {
			continue
		}
		seen[jobURL] = struct{}{}

		title := "Gradcracker Opportunity"
		if i < len(titles) {
			title = strings.TrimSpace(titles[i][1])
		}
		company := "Unknown Employer"
		if i < len(employers) {
			company = strings.TrimSpace(employers[i][1])
		}
		loc := ""
		if i < len(locations) {
			loc = strings.TrimSpace(locations[i][1])
		}
		desc := ""
		if i < len(deadlines) {
			desc = "Deadline: " + strings.TrimSpace(deadlines[i][1])
		}
		if i < len(salaries) && strings.TrimSpace(salaries[i][1]) != "" {
			if desc != "" {
				desc += " | "
			}
			desc += "Salary: " + strings.TrimSpace(salaries[i][1])
		}

		id := util.NormalizeSlug(jobURL)
		if idx := strings.LastIndex(jobURL, "/"); idx >= 0 && idx+1 < len(jobURL) {
			id = util.NormalizeSlug(jobURL[idx+1:])
		}
		jobs = append(jobs, model.JobPost{ID: "gc-" + id, Title: title, CompanyName: company, Location: model.Location{City: loc}, JobURL: jobURL, Description: desc})
	}
	return jobs
}

func limitGradcrackerJobs(in []model.JobPost, wanted int) []model.JobPost {
	if wanted <= 0 || wanted > len(in) {
		return in
	}
	return in[:wanted]
}
