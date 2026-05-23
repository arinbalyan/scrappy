package arbeitnow

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

const apiURL = "https://www.arbeitnow.com/api/job-board-api"

const maxPages = 3

var anStripTags = regexp.MustCompile(`(?is)<[^>]+>`)

type apiResp struct {
	Data  []apiJob `json:"data"`
	Links struct {
		Next *string `json:"next"`
	} `json:"links"`
}

type apiJob struct {
	Slug        string   `json:"slug"`
	CompanyName string   `json:"company_name"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Remote      bool     `json:"remote"`
	URL         string   `json:"url"`
	Tags        []string `json:"tags"`
	Location    string   `json:"location"`
	CreatedAt   int64    `json:"created_at"`
}

type Scraper struct {
	client  *http.Client
	baseURL string
}

func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Retries: 2, Timeout: 20 * time.Second})
	}
	return &Scraper{client: client, baseURL: apiURL}
}

func NewWithBaseURL(client *http.Client, baseURL string) *Scraper {
	s := New(client)
	if strings.TrimSpace(baseURL) != "" {
		s.baseURL = strings.TrimSpace(baseURL)
	}
	return s
}

func (s *Scraper) SiteName() model.Site { return model.SiteArbeitnow }

func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	wanted := input.ResultsWanted
	if wanted <= 0 {
		wanted = 25
	}

	term := strings.ToLower(strings.TrimSpace(input.SearchTerm))
	jobs := make([]model.JobPost, 0, wanted)

	for page := 1; page <= maxPages && len(jobs) < wanted; page++ {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, s.baseURL, nil)
		q := req.URL.Query()
		q.Set("page", fmt.Sprintf("%d", page))
		req.URL.RawQuery = q.Encode()
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "Mozilla/5.0")

		resp, err := s.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("arbeitnow request: %w", err)
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			resp.Body.Close()
			return nil, fmt.Errorf("arbeitnow status %d", resp.StatusCode)
		}
		body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("arbeitnow read: %w", err)
		}

		var parsed apiResp
		if err := json.Unmarshal(body, &parsed); err != nil {
			return nil, fmt.Errorf("arbeitnow decode: %w", err)
		}
		if len(parsed.Data) == 0 {
			break
		}

		for _, r := range parsed.Data {
			if len(jobs) >= wanted {
				break
			}
			if strings.TrimSpace(r.Title) == "" || strings.TrimSpace(r.URL) == "" {
				continue
			}
			if term != "" {
				hay := strings.ToLower(r.Title + " " + r.Description + " " + strings.Join(r.Tags, " "))
				if !strings.Contains(hay, term) {
					continue
				}
			}

			job := model.JobPost{
				ID:          "arbeitnow-" + strings.TrimSpace(r.Slug),
				Title:       strings.TrimSpace(r.Title),
				CompanyName: strings.TrimSpace(r.CompanyName),
				JobURL:      strings.TrimSpace(r.URL),
				Description: anHTMLToText(r.Description),
				IsRemote:    r.Remote,
				Location: model.Location{
					City: strings.TrimSpace(r.Location),
				},
			}
			if strings.TrimSpace(job.ID) == "arbeitnow-" {
				job.ID = "arbeitnow-" + idFromURL(job.JobURL)
			}
			if r.CreatedAt > 0 {
				t := time.Unix(r.CreatedAt, 0).UTC()
				job.DatePosted = &t
			}
			jobs = append(jobs, job)
		}

		if parsed.Links.Next == nil {
			break
		}
	}

	if !util.HasMeaningfulJobs(jobs) {
		return nil, fmt.Errorf("arbeitnow no parseable jobs")
	}
	return jobs, nil
}

func anHTMLToText(v string) string {
	if strings.TrimSpace(v) == "" {
		return ""
	}
	v = anStripTags.ReplaceAllString(v, " ")
	return strings.Join(strings.Fields(strings.TrimSpace(v)), " ")
}

func idFromURL(raw string) string {
	u := strings.TrimSpace(raw)
	if u == "" {
		return "unknown"
	}
	parts := strings.Split(u, "/")
	for i := len(parts) - 1; i >= 0; i-- {
		p := strings.TrimSpace(parts[i])
		if p != "" {
			return p
		}
	}
	return "unknown"
}
