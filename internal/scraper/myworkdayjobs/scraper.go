package myworkdayjobs

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/util"
)

type Scraper struct{ client *http.Client }

func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Retries: 2, CookieResetEveryN: 120, Timeout: 25 * time.Second})
	}
	return &Scraper{client: client}
}

func (s *Scraper) SiteName() model.Site { return model.SiteMyWorkdayJobs }

func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{"site": s.SiteName(), "results_wanted": input.ResultsWanted, "search_term": input.SearchTerm, "location": input.Location})

	seeds := normalizeSeeds(input.WorkdaySeeds)
	if len(seeds) == 0 {
		util.APIMiss("workday_no_seeds", map[string]any{"site": model.SiteMyWorkdayJobs})
		return nil, fmt.Errorf("workday missing seeds: set --workday-seeds or SCRAPPY_WORKDAY_SEEDS")
	}

	if strings.TrimSpace(input.SearchTerm) == "" {
		return nil, fmt.Errorf("workday missing search term")
	}
	util.Debug("workday_scrape_begin", map[string]any{"seeds": len(seeds), "results_wanted": input.ResultsWanted})
	out := make([]model.JobPost, 0, input.ResultsWanted)
	seen := map[string]struct{}{}
	seedErrs := make([]error, 0)
	successfulSeeds := 0
	for _, seed := range seeds {
		jobs, err := s.fetchSeedJobs(ctx, seed)
		if err != nil {
			util.Warn("workday_seed_failed", map[string]any{"seed": seed, "err": err.Error()})
			seedErrs = append(seedErrs, fmt.Errorf("%s: %w", seed, err))
			continue
		}
		util.Debug("workday_seed_success", map[string]any{"seed": seed, "jobs": len(jobs)})
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
	body := []byte(`{"limit":20,"offset":0,"searchText":""}`)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, seed, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("workday status %d", resp.StatusCode)
	}
	raw, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return nil, err
	}
	var parsed struct {
		JobPostings []struct {
			BulletFields   []string `json:"bulletFields"`
			Title          string   `json:"title"`
			ExternalPath   string   `json:"externalPath"`
			PostedOn       string   `json:"postedOn"`
			LocationsText  string   `json:"locationsText"`
			JobDescription string   `json:"jobDescription"`
			RemoteType     string   `json:"remoteType"`
		} `json:"jobPostings"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, err
	}
	jobs := make([]model.JobPost, 0, len(parsed.JobPostings))
	for i, r := range parsed.JobPostings {
		jobURL := seed
		if strings.TrimSpace(r.ExternalPath) != "" {
			jobURL = deriveApplyURL(seed, r.ExternalPath)
		}
		desc := strings.TrimSpace(r.JobDescription)
		exp := inferExperience(append([]string{r.Title, desc}, r.BulletFields...)...)
		post := model.JobPost{
			ID:              fmt.Sprintf("wd-%d-%s", i, util.NormalizeSlug(r.Title)),
			Title:           strings.TrimSpace(r.Title),
			CompanyName:     deriveCompany(seed),
			JobURL:          jobURL,
			Description:     desc,
			Location:        parseLocation(r.LocationsText),
			IsRemote:        strings.Contains(strings.ToLower(r.RemoteType+" "+r.LocationsText), "remote"),
			ExperienceRange: exp,
		}
		post.DatePosted = util.ParseDatePosted(r.PostedOn)
		jobs = append(jobs, post)
	}
	util.Debug("scraper_done", map[string]any{"site": s.SiteName(), "jobs": len(jobs)})
	return jobs, nil
}

func normalizeSeeds(in []string) []string {
	if len(in) == 0 {
		if v := strings.TrimSpace(os.Getenv("SCRAPPY_WORKDAY_SEEDS")); v != "" {
			in = strings.Split(v, ",")
		}
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		v := strings.TrimSpace(s)
		if v == "" {
			continue
		}
		if !strings.Contains(v, "/wday/cxs/") {
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

func deriveApplyURL(seed, path string) string {
	u, err := url.Parse(seed)
	if err != nil {
		return seed
	}
	u.Path = strings.TrimSuffix(strings.Split(u.Path, "/wday/cxs/")[0], "/") + "/" + strings.TrimLeft(path, "/")
	u.RawQuery = ""
	return u.String()
}

func deriveCompany(seed string) string {
	u, err := url.Parse(seed)
	if err != nil || u == nil {
		return ""
	}
	host := u.Hostname()
	parts := strings.Split(host, ".")
	if len(parts) > 0 {
		return parts[0]
	}
	return host
}

func parseLocation(v string) model.Location {
	parts := strings.Split(v, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	switch len(parts) {
	case 0:
		return model.Location{}
	case 1:
		return model.Location{City: parts[0]}
	case 2:
		return model.Location{City: parts[0], State: parts[1]}
	default:
		return model.Location{City: parts[0], State: parts[1], Country: parts[2]}
	}
}

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

var reYears = regexp.MustCompile(`(?i)(\d+\+?\s*(?:years|yrs))`)

func inferExperience(values ...string) string {
	for _, v := range values {
		if m := reYears.FindString(v); m != "" {
			return strings.TrimSpace(m)
		}
	}
	return ""
}
