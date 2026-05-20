package google

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/util"
)

const defaultSearchURL = "https://www.google.com/search"

var (
	reGoogleJob = regexp.MustCompile(`(?s)data-job-id="([^"]+)"[\s\S]*?<div[^>]*class="BjJfJf PUpOsf">([^<]+)</div>[\s\S]*?<div[^>]*class="Qk80Jf">([^<]+)</div>`)
	reGoogleLD  = regexp.MustCompile(`(?s)<script[^>]*type="application/ld\+json"[^>]*>(.*?)</script>`)
)

type Scraper struct { client *http.Client; searchURL string }
func New(client *http.Client) *Scraper { if client == nil { client = util.NewHTTPClient(util.ClientOptions{Retries: 2, CookieResetEveryN: 100}) }; return &Scraper{client: client, searchURL: defaultSearchURL} }
func NewWithSearchURL(client *http.Client, u string) *Scraper { s := New(client); if strings.TrimSpace(u) != "" { s.searchURL = u }; return s }
func (s *Scraper) SiteName() model.Site { return model.SiteGoogle }

func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	u, _ := url.Parse(s.searchURL)
	q := u.Query()
	if input.GoogleSearchTerm != "" {
		q.Set("q", input.GoogleSearchTerm+" jobs")
	} else if input.SearchTerm != "" {
		q.Set("q", input.SearchTerm+" jobs")
	}
	u.RawQuery = q.Encode()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("google request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("google status %d", resp.StatusCode)
	}
	b, _ := io.ReadAll(resp.Body)

	if jobs := parseLDJSONJobs(string(b)); len(jobs) > 0 {
		return limitJobs(jobs, input.ResultsWanted), nil
	}
	return limitJobs(parseHTMLJobs(string(b)), input.ResultsWanted), nil
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
