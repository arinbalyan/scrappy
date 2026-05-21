package google

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/util"
)

const defaultSearchURL = "https://www.google.com/search"

var (
	reGoogleJob       = regexp.MustCompile(`(?s)data-job-id="([^"]+)"[\s\S]*?<div[^>]*class="BjJfJf PUpOsf">([^<]+)</div>[\s\S]*?<div[^>]*class="Qk80Jf">([^<]+)</div>`)
	reGoogleLD        = regexp.MustCompile(`(?s)<script[^>]*type="application/ld\+json"[^>]*>(.*?)</script>`)
	reGoogleInitial52 = regexp.MustCompile(`"520084652":(\[.*?\]\s*])\s*}\s*]\s*]\s*]\s*]\s*]`)
	reGoogleFC        = regexp.MustCompile(`data-async-fc="([^"]+)"`)
)

type Scraper struct {
	client    *http.Client
	searchURL string
}

func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Retries: 2, CookieResetEveryN: 100})
	}
	return &Scraper{client: client, searchURL: defaultSearchURL}
}
func NewWithSearchURL(client *http.Client, u string) *Scraper {
	s := New(client)
	if strings.TrimSpace(u) != "" {
		s.searchURL = u
	}
	return s
}
func (s *Scraper) SiteName() model.Site { return model.SiteGoogle }

func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{"site": s.SiteName(), "results_wanted": input.ResultsWanted, "search_term": input.SearchTerm, "location": input.Location})

	u, _ := url.Parse(s.searchURL)
	q := u.Query()
	query := strings.TrimSpace(input.GoogleSearchTerm)
	if query == "" {
		query = strings.TrimSpace(input.SearchTerm)
		if query != "" {
			query += " jobs"
		}
	}
	q.Set("q", query)
	q.Set("udm", "8")
	q.Set("hl", "en")
	u.RawQuery = q.Encode()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	req.Header.Set("accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("accept-language", "en-US,en;q=0.9")
	req.Header.Set("user-agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("google request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("google status %d", resp.StatusCode)
	}
	b, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("google read: %w", err)
	}

	raw := string(b)
	if jobs := parseGoogleInitialJobs(raw); len(jobs) > 0 {
		limited := limitJobs(jobs, input.ResultsWanted)
		if util.HasMeaningfulJobs(limited) {
			return limited, nil
		}
	}
	if jobs := parseLDJSONJobs(raw); len(jobs) > 0 {
		limited := limitJobs(jobs, input.ResultsWanted)
		if util.HasMeaningfulJobs(limited) {
			return limited, nil
		}
	}
	limited := limitJobs(parseHTMLJobs(raw), input.ResultsWanted)
	if !util.HasMeaningfulJobs(limited) {
		return nil, fmt.Errorf("google no parseable job cards from response")
	}
	return limited, nil
}

func limitJobs(in []model.JobPost, wanted int) []model.JobPost {
	if wanted <= 0 || wanted > len(in) {
		return in
	}
	return in[:wanted]
}

func parseHTMLJobs(raw string) []model.JobPost {
	m := reGoogleJob.FindAllStringSubmatch(raw, -1)
	out := make([]model.JobPost, 0, len(m))
	for _, row := range m {
		out = append(out, model.JobPost{
			ID:          "go-" + row[1],
			Title:       strings.TrimSpace(row[2]),
			CompanyName: strings.TrimSpace(row[3]),
			JobURL:      "https://www.google.com/search?q=" + url.QueryEscape(row[2]),
		})
	}
	return out
}

func parseLDJSONJobs(raw string) []model.JobPost {
	type ldJob struct {
		Type        string `json:"@type"`
		Title       string `json:"title"`
		Description string `json:"description"`
		DatePosted  string `json:"datePosted"`
		HiringOrg   struct {
			Name string `json:"name"`
		} `json:"hiringOrganization"`
	}
	type ldGraph struct {
		Type  string  `json:"@type"`
		Graph []ldJob `json:"@graph"`
	}

	scripts := reGoogleLD.FindAllStringSubmatch(raw, -1)
	jobs := make([]model.JobPost, 0)
	for i, s := range scripts {
		body := strings.TrimSpace(s[1])
		if body == "" {
			continue
		}

		var single ldJob
		if err := json.Unmarshal([]byte(body), &single); err == nil && strings.EqualFold(single.Type, "JobPosting") {
			jobs = append(jobs, ldJobToPost(single.Title, single.HiringOrg.Name, single.Description, single.DatePosted, i))
			continue
		}

		var many []ldJob
		if err := json.Unmarshal([]byte(body), &many); err == nil {
			for idx, j := range many {
				if !strings.EqualFold(j.Type, "JobPosting") {
					continue
				}
				jobs = append(jobs, ldJobToPost(j.Title, j.HiringOrg.Name, j.Description, j.DatePosted, i*1000+idx))
			}
			continue
		}

		var graph ldGraph
		if err := json.Unmarshal([]byte(body), &graph); err == nil {
			for idx, j := range graph.Graph {
				if !strings.EqualFold(j.Type, "JobPosting") {
					continue
				}
				jobs = append(jobs, ldJobToPost(j.Title, j.HiringOrg.Name, j.Description, j.DatePosted, i*1000+idx))
			}
		}
	}
	return jobs
}

func parseGoogleInitialJobs(raw string) []model.JobPost {
	matches := reGoogleInitial52.FindAllStringSubmatch(raw, -1)
	out := make([]model.JobPost, 0, len(matches))
	for i, m := range matches {
		if len(m) < 2 {
			continue
		}
		var parsed []any
		if err := json.Unmarshal([]byte(m[1]), &parsed); err != nil {
			continue
		}
		if job := toGoogleInitialJob(parsed, i); job != nil {
			out = append(out, *job)
		}
	}
	return out
}

func toGoogleInitialJob(v []any, seed int) *model.JobPost {
	if len(v) < 4 {
		return nil
	}
	title, _ := v[0].(string)
	company, _ := v[1].(string)
	loc, _ := v[2].(string)
	if strings.TrimSpace(title) == "" {
		return nil
	}
	jobURL := ""
	if refs, ok := v[3].([]any); ok && len(refs) > 0 {
		if first, ok := refs[0].([]any); ok && len(first) > 0 {
			jobURL, _ = first[0].(string)
		}
	}
	if strings.TrimSpace(jobURL) == "" {
		jobURL = "https://www.google.com/search?q=" + url.QueryEscape(title+" "+company)
	}
	jp := model.JobPost{ID: fmt.Sprintf("go-%s-%s-%d", util.NormalizeSlug(title), util.NormalizeSlug(company), seed), Title: strings.TrimSpace(title), CompanyName: strings.TrimSpace(company), JobURL: strings.TrimSpace(jobURL), Location: parseSimpleLocation(loc)}
	jp.IsRemote = strings.Contains(strings.ToLower(title+" "+loc), "remote")
	return &jp
}

func parseSimpleLocation(v string) model.Location {
	parts := strings.Split(v, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	switch len(parts) {
	case 1:
		return model.Location{City: parts[0]}
	case 2:
		return model.Location{City: parts[0], State: parts[1]}
	case 3:
		return model.Location{City: parts[0], State: parts[1], Country: parts[2]}
	default:
		return model.Location{}
	}
}

func ldJobToPost(title, company, description, datePosted string, seed int) model.JobPost {
	post := model.JobPost{
		ID:          fmt.Sprintf("go-%s-%s-%d", util.NormalizeSlug(title), util.NormalizeSlug(company), seed),
		Title:       strings.TrimSpace(title),
		CompanyName: strings.TrimSpace(company),
		Description: strings.TrimSpace(description),
		JobURL:      "https://www.google.com/search?q=" + url.QueryEscape(strings.TrimSpace(title)+" "+strings.TrimSpace(company)),
	}
	post.DatePosted = util.ParseDatePosted(datePosted)
	return post
}
