package workable_jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/util"
)

const defaultBase = "https://apply.workable.com"

type Scraper struct {
	client *http.Client
	base   string
}

func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Retries: 2, CookieResetEveryN: 120, Timeout: 20 * time.Second})
	}
	return &Scraper{client: client, base: defaultBase}
}

func NewWithBaseURL(client *http.Client, base string) *Scraper {
	s := New(client)
	if strings.TrimSpace(base) != "" {
		s.base = strings.TrimRight(strings.TrimSpace(base), "/")
	}
	return s
}

func (s *Scraper) SiteName() model.Site { return model.SiteWorkableJobs }

func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{"site": s.SiteName(), "results_wanted": input.ResultsWanted, "search_term": input.SearchTerm, "location": input.Location})

	seeds := normalizeSeeds(input.WorkableSeeds, "SCRAPPY_WORKABLE_SEEDS")
	if len(seeds) == 0 {
		util.APIMiss("workable_no_seeds", map[string]any{"site": model.SiteWorkableJobs})
		return nil, nil
	}
	util.Debug("workable_scrape_begin", map[string]any{"seeds": len(seeds), "results_wanted": input.ResultsWanted})
	out := make([]model.JobPost, 0, input.ResultsWanted)
	seen := map[string]struct{}{}
	seedErrs := make([]error, 0)
	successfulSeeds := 0
	for _, seed := range seeds {
		jobs, err := s.fetchSeedJobs(ctx, seed)
		if err != nil {
			util.Warn("workable_seed_failed", map[string]any{"seed": seed, "err": err.Error()})
			seedErrs = append(seedErrs, fmt.Errorf("%s: %w", seed, err))
			continue
		}
		util.Debug("workable_seed_success", map[string]any{"seed": seed, "jobs": len(jobs)})
		successfulSeeds++
		for _, j := range jobs {
			if _, ok := seen[j.JobURL]; ok {
				continue
			}
			seen[j.JobURL] = struct{}{}
			if !matchRole(input.SearchTerm, j.Title, j.Description) {
				continue
			}
			out = append(out, j)
			if input.ResultsWanted > 0 && len(out) >= input.ResultsWanted {
				return out, nil
			}
		}
	}
	if successfulSeeds == 0 && len(seedErrs) > 0 {
		return nil, errors.Join(seedErrs...)
	}
	util.Debug("scraper_done", map[string]any{"site": s.SiteName(), "jobs": len(out)})
	return out, nil
}

func (s *Scraper) fetchSeedJobs(ctx context.Context, seed string) ([]model.JobPost, error) {
	u := fmt.Sprintf("%s/api/v1/widget/accounts/%s/jobs", strings.TrimRight(s.base, "/"), url.PathEscape(seed))
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	req.Header.Set("Accept", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("workable status %d", resp.StatusCode)
	}
	body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return nil, err
	}
	var parsed struct {
		Results []struct {
			ID        string `json:"id"`
			Title     string `json:"title"`
			URL       string `json:"url"`
			Shortcode string `json:"shortcode"`
			Code      string `json:"code"`
			Location  struct {
				LocationStr string `json:"location_str"`
			} `json:"location"`
			Remote         bool   `json:"remote"`
			Description    string `json:"description"`
			CreatedAt      string `json:"created_at"`
			EmploymentType string `json:"employment_type"`
			Department     string `json:"department"`
		} `json:"results"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}
	jobs := make([]model.JobPost, 0, len(parsed.Results))
	for _, r := range parsed.Results {
		jobURL := strings.TrimSpace(r.URL)
		if jobURL == "" && r.Shortcode != "" {
			jobURL = fmt.Sprintf("%s/%s/j/%s", strings.TrimRight(s.base, "/"), seed, r.Shortcode)
		}
		loc := parseLocation(r.Location.LocationStr)
		post := model.JobPost{
			ID:          fmt.Sprintf("wk-%s-%s", seed, fallback(r.ID, r.Code)),
			Title:       strings.TrimSpace(r.Title),
			CompanyName: seed,
			JobURL:      jobURL,
			Description: strings.TrimSpace(r.Description),
			Location:    loc,
			IsRemote:    r.Remote || strings.Contains(strings.ToLower(r.Location.LocationStr), "remote"),
			Department:  strings.TrimSpace(r.Department),
			JobType:     strings.ToLower(strings.TrimSpace(r.EmploymentType)),
		}
		post.DatePosted = util.ParseDatePosted(r.CreatedAt)
		jobs = append(jobs, post)
	}
	util.Debug("scraper_done", map[string]any{"site": s.SiteName(), "jobs": len(jobs)})
	return jobs, nil
}

func normalizeSeeds(in []string, envName string) []string {
	if len(in) == 0 {
		if v := strings.TrimSpace(getenv(envName)); v != "" {
			in = strings.Split(v, ",")
		}
	}
	out := make([]string, 0, len(in))
	seen := map[string]struct{}{}
	for _, s := range in {
		v := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(s, "https://apply.workable.com/"), "http://apply.workable.com/"))
		v = strings.Trim(v, "/")
		if strings.Contains(v, "/") {
			v = strings.Split(v, "/")[0]
		}
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

var getenv = os.Getenv

func matchRole(role, title, description string) bool {
	role = strings.TrimSpace(strings.ToLower(role))
	if role == "" {
		return true
	}
	hay := strings.ToLower(title + " " + description)
	if strings.Contains(hay, role) {
		return true
	}
	for _, syn := range roleSynonyms(role) {
		if strings.Contains(hay, syn) {
			return true
		}
	}
	return false
}

func roleSynonyms(role string) []string {
	m := map[string][]string{
		"software engineer": {"software developer", "backend engineer", "frontend engineer", "full stack"},
		"data scientist":    {"ml engineer", "machine learning", "data analyst"},
	}
	return m[role]
}

func parseLocation(v string) model.Location {
	parts := strings.Split(v, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	if len(parts) == 0 {
		return model.Location{}
	}
	if len(parts) == 1 {
		return model.Location{City: parts[0]}
	}
	if len(parts) == 2 {
		return model.Location{City: parts[0], State: parts[1]}
	}
	return model.Location{City: parts[0], State: parts[1], Country: parts[2]}
}

func fallback(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}
