package hiringcafe

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

const baseURL = "https://hiring.cafe/"

var reNextData = regexp.MustCompile(`(?s)<script[^>]*id="__NEXT_DATA__"[^>]*type="application/json"[^>]*>(.*?)</script>`)

type Scraper struct {
	client  *http.Client
	baseURL string
}

func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Retries: 2, Timeout: 15 * time.Second})
	}
	return &Scraper{client: client, baseURL: baseURL}
}

func NewWithBaseURL(client *http.Client, endpoint string) *Scraper {
	s := New(client)
	if strings.TrimSpace(endpoint) != "" {
		s.baseURL = strings.TrimRight(endpoint, "/") + "/"
	}
	return s
}

func (s *Scraper) SiteName() model.Site { return model.SiteHiringCafe }

func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{"site": s.SiteName(), "results_wanted": input.ResultsWanted})

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.baseURL, nil)
	if err != nil {
		return nil, fmt.Errorf("hiringcafe: build request: %w", err)
	}
	req.Header.Set("accept", "text/html,application/xhtml+xml")
	req.Header.Set("user-agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("hiringcafe: request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("hiringcafe status %d", resp.StatusCode)
	}

	body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("hiringcafe: read: %w", err)
	}

	jobs, err := parseJobs(body, input)
	if err != nil {
		return nil, err
	}

	util.Debug("scraper_done", map[string]any{"site": s.SiteName(), "jobs": len(jobs)})
	return jobs, nil
}

func parseJobs(body []byte, input model.ScraperInput) ([]model.JobPost, error) {
	html := string(body)
	m := reNextData.FindStringSubmatch(html)
	if len(m) < 2 {
		return nil, fmt.Errorf("hiringcafe: no __NEXT_DATA__ found")
	}

	var pageData struct {
		Props struct {
			PageProps struct {
				Hits []struct {
					ApplyURL   string `json:"apply_url"`
					JobInfo    struct {
						Title    string          `json:"title"`
						RawTitle string          `json:"job_title_raw"`
						Extra    json.RawMessage `json:"-"`
					} `json:"job_information"`
					CompanyData struct {
						Name string `json:"name"`
					} `json:"enriched_company_data"`
				} `json:"ssrHits"`
			} `json:"pageProps"`
		} `json:"props"`
	}

	if err := json.Unmarshal([]byte(m[1]), &pageData); err != nil {
		return nil, fmt.Errorf("hiringcafe: decode __NEXT_DATA__: %w", err)
	}

	hits := pageData.Props.PageProps.Hits
	if len(hits) == 0 {
		return nil, fmt.Errorf("hiringcafe: no jobs found")
	}

	wanted := input.ResultsWanted
	if wanted <= 0 || wanted > len(hits) {
		wanted = len(hits)
	}

	out := make([]model.JobPost, 0, wanted)
	for _, h := range hits[:wanted] {
		title := strings.TrimSpace(h.JobInfo.Title)
		if title == "" {
			title = strings.TrimSpace(h.JobInfo.RawTitle)
		}
		if title == "" {
			continue
		}

		url := strings.TrimSpace(h.ApplyURL)
		if url == "" {
			continue
		}

		company := strings.TrimSpace(h.CompanyData.Name)

		out = append(out, model.JobPost{
			ID:          "hc-" + util.HashID(url),
			Title:       title,
			CompanyName: company,
			JobURL:      url,
			Site:        string(model.SiteHiringCafe),
		})
	}

	if len(out) == 0 {
		return nil, fmt.Errorf("hiringcafe: no parseable jobs")
	}
	return out, nil
}
