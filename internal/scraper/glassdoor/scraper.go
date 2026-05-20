package glassdoor

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/util"
)

const defaultURL = "https://www.glassdoor.com/Job/jobs.htm"

var (
	reJob      = regexp.MustCompile(`(?s)data-jobid="([^"]+)"[\s\S]*?<a[^>]*class="jobLink"[^>]*>([^<]+)</a>[\s\S]*?<span[^>]*class="EmployerProfile_compactEmployerName"[^>]*>([^<]+)</span>`)
	reLDScript = regexp.MustCompile(`(?s)<script[^>]*type="application/ld\+json"[^>]*>(.*?)</script>`)
)

type Scraper struct {
	client  *http.Client
	listURL string
}

func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Retries: 2, CookieResetEveryN: 100})
	}
	return &Scraper{client: client, listURL: defaultURL}
}
func NewWithListURL(client *http.Client, u string) *Scraper {
	s := New(client)
	if strings.TrimSpace(u) != "" {
		s.listURL = u
	}
	return s
}
func (s *Scraper) SiteName() model.Site { return model.SiteGlassdoor }

func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{"site": s.SiteName(), "results_wanted": input.ResultsWanted, "search_term": input.SearchTerm, "location": input.Location})

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, s.listURL, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("glassdoor request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("glassdoor status %d", resp.StatusCode)
	}
	b, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("glassdoor read: %w", err)
	}
	raw := string(b)

	if jobs := parseLDJSONJobs(raw); len(jobs) > 0 {
		limited := limitJobs(jobs, input.ResultsWanted)
		if util.HasMeaningfulJobs(limited) {
			return limited, nil
		}
	}
	limited := limitJobs(parseHTMLJobs(raw, s.listURL), input.ResultsWanted)
	if !util.HasMeaningfulJobs(limited) {
		return nil, nil
	}
	return limited, nil
}

func limitJobs(in []model.JobPost, wanted int) []model.JobPost {
	if wanted <= 0 || wanted > len(in) {
		return in
	}
	return in[:wanted]
}

func parseHTMLJobs(raw, sourceURL string) []model.JobPost {
	m := reJob.FindAllStringSubmatch(raw, -1)
	out := make([]model.JobPost, 0, len(m))
	for _, row := range m {
		out = append(out, model.JobPost{
			ID:          "gd-" + row[1],
			Title:       strings.TrimSpace(row[2]),
			CompanyName: strings.TrimSpace(row[3]),
			JobURL:      sourceURL,
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
		URL         string `json:"url"`
		HiringOrg   struct {
			Name string `json:"name"`
		} `json:"hiringOrganization"`
	}
	type ldGraph struct {
		Graph []ldJob `json:"@graph"`
	}

	scripts := reLDScript.FindAllStringSubmatch(raw, -1)
	jobs := make([]model.JobPost, 0)
	for i, s := range scripts {
		body := strings.TrimSpace(s[1])
		if body == "" {
			continue
		}
		var single ldJob
		if err := json.Unmarshal([]byte(body), &single); err == nil && strings.EqualFold(single.Type, "JobPosting") {
			jobs = append(jobs, toPost(single.Title, single.HiringOrg.Name, single.Description, single.URL, single.DatePosted, i))
			continue
		}
		var many []ldJob
		if err := json.Unmarshal([]byte(body), &many); err == nil {
			for idx, j := range many {
				if strings.EqualFold(j.Type, "JobPosting") {
					jobs = append(jobs, toPost(j.Title, j.HiringOrg.Name, j.Description, j.URL, j.DatePosted, i*1000+idx))
				}
			}
			continue
		}
		var graph ldGraph
		if err := json.Unmarshal([]byte(body), &graph); err == nil {
			for idx, j := range graph.Graph {
				if strings.EqualFold(j.Type, "JobPosting") {
					jobs = append(jobs, toPost(j.Title, j.HiringOrg.Name, j.Description, j.URL, j.DatePosted, i*1000+idx))
				}
			}
		}
	}
	return jobs
}

func toPost(title, company, description, jobURL, datePosted string, seed int) model.JobPost {
	title = strings.TrimSpace(title)
	company = strings.TrimSpace(company)
	post := model.JobPost{
		ID:          fmt.Sprintf("gd-%s-%s-%d", util.NormalizeSlug(title), util.NormalizeSlug(company), seed),
		Title:       title,
		CompanyName: company,
		Description: strings.TrimSpace(description),
		JobURL:      strings.TrimSpace(jobURL),
	}
	if post.JobURL == "" {
		post.JobURL = defaultURL
	}
	post.DatePosted = util.ParseDatePosted(datePosted)
	return post
}
