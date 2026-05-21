package hiringcafe

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/util"
)

const defaultAPI = "https://hiring.cafe/"

var reNextData = regexp.MustCompile(`(?s)<script[^>]*id=["']__NEXT_DATA__["'][^>]*>(.*?)</script>`)

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
func (s *Scraper) SiteName() model.Site { return model.SiteHiringCafe }

func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{"site": s.SiteName(), "results_wanted": input.ResultsWanted, "search_term": input.SearchTerm, "location": input.Location})

	u, _ := url.Parse(s.apiURL)
	q := u.Query()
	q.Set("searchState", `{"searchQuery":"`+strings.TrimSpace(input.SearchTerm)+`"}`)
	u.RawQuery = q.Encode()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("hiringcafe request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("hiringcafe status %d", resp.StatusCode)
	}
	body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("hiringcafe read: %w", err)
	}
	jobs := parseAPIJobs(body)
	if !util.HasMeaningfulJobs(jobs) {
		jobs = parseNextData(string(body))
	}
	jobs = limitHiringCafeJobs(jobs, input.ResultsWanted)
	if !util.HasMeaningfulJobs(jobs) {
		return nil, fmt.Errorf("hiringcafe no parseable jobs")
	}
	util.Debug("scraper_done", map[string]any{"site": s.SiteName(), "jobs": len(jobs)})
	return jobs, nil
}

func parseAPIJobs(body []byte) []model.JobPost {
	var parsed []struct {
		ID, Title, Company, Location, URL, Description, PostedAt string
		Remote                                                   bool
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil
	}
	out := make([]model.JobPost, 0, len(parsed))
	for i, r := range parsed {
		title := strings.TrimSpace(r.Title)
		company := strings.TrimSpace(r.Company)
		if title == "" || company == "" {
			continue
		}
		post := model.JobPost{ID: "hc-" + strings.TrimSpace(r.ID), Title: title, CompanyName: company, Location: model.Location{City: strings.TrimSpace(r.Location)}, JobURL: strings.TrimSpace(r.URL), Description: strings.TrimSpace(r.Description), IsRemote: r.Remote || strings.Contains(strings.ToLower(r.Location), "remote")}
		if post.ID == "hc-" {
			post.ID = fmt.Sprintf("hc-%d", i+1)
		}
		if post.JobURL == "" {
			post.JobURL = defaultAPI
		}
		if t, err := time.Parse(time.RFC3339, strings.TrimSpace(r.PostedAt)); err == nil {
			post.DatePosted = &t
		}
		out = append(out, post)
	}
	return out
}

func parseNextData(raw string) []model.JobPost {
	m := reNextData.FindStringSubmatch(raw)
	if len(m) < 2 {
		return nil
	}
	var parsed struct {
		Props struct {
			PageProps struct {
				SSRHits []struct {
					ID       string `json:"id"`
					ApplyURL string `json:"apply_url"`
					JobInfo  struct {
						Title       string `json:"title"`
						Description string `json:"description"`
						CompanyInfo struct {
							Name string `json:"name"`
						} `json:"company_info"`
					} `json:"job_information"`
					Data struct {
						Location      string `json:"formatted_workplace_location"`
						WorkplaceType string `json:"workplace_type"`
						PublishDate   string `json:"estimated_publish_date"`
					} `json:"v5_processed_job_data"`
				} `json:"ssrHits"`
			} `json:"pageProps"`
		} `json:"props"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(m[1])), &parsed); err != nil {
		return nil
	}
	rows := parsed.Props.PageProps.SSRHits
	out := make([]model.JobPost, 0, len(rows))
	for i, r := range rows {
		title := strings.TrimSpace(r.JobInfo.Title)
		company := strings.TrimSpace(r.JobInfo.CompanyInfo.Name)
		if title == "" || company == "" {
			continue
		}
		post := model.JobPost{ID: "hc-" + strings.TrimSpace(r.ID), Title: title, CompanyName: company, JobURL: strings.TrimSpace(r.ApplyURL), Description: strings.TrimSpace(r.JobInfo.Description), Location: model.Location{City: strings.TrimSpace(r.Data.Location)}, DatePosted: util.ParseDatePosted(r.Data.PublishDate), IsRemote: strings.EqualFold(strings.TrimSpace(r.Data.WorkplaceType), "Remote")}
		if post.ID == "hc-" {
			post.ID = fmt.Sprintf("hc-%d", i+1)
		}
		if post.JobURL == "" {
			post.JobURL = defaultAPI
		}
		out = append(out, post)
	}
	return out
}

func limitHiringCafeJobs(in []model.JobPost, wanted int) []model.JobPost {
	if wanted <= 0 || wanted > len(in) {
		return in
	}
	return in[:wanted]
}
