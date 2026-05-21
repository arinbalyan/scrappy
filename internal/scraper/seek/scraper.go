package seek

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

const (
	defaultAPI  = "https://www.seek.com.au/api/chalice-search/v4/search"
	defaultList = "https://www.seek.com.au/jobs"
)

var reLDScript = regexp.MustCompile(`(?s)<script[^>]*type="application/ld\+json"[^>]*>(.*?)</script>`)

type Scraper struct {
	client  *http.Client
	apiURL  string
	listURL string
}

func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Retries: 3, CookieResetEveryN: 160, Timeout: 20 * time.Second})
	}
	return &Scraper{client: client, apiURL: defaultAPI, listURL: defaultList}
}

func NewWithAPIURL(client *http.Client, endpoint string) *Scraper {
	s := New(client)
	if strings.TrimSpace(endpoint) != "" {
		s.apiURL = endpoint
	}
	return s
}

func (s *Scraper) SiteName() model.Site { return model.SiteSeek }

func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{"site": s.SiteName(), "results_wanted": input.ResultsWanted, "search_term": input.SearchTerm, "location": input.Location})

	jobs, err := s.scrapeAPI(ctx, input)
	if err == nil && util.HasMeaningfulJobs(jobs) {
		util.Debug("scraper_done", map[string]any{"site": s.SiteName(), "jobs": len(jobs), "path": "api"})
		return jobs, nil
	}

	fallback, ferr := s.scrapeHTML(ctx, input)
	if ferr == nil && util.HasMeaningfulJobs(fallback) {
		util.Debug("scraper_done", map[string]any{"site": s.SiteName(), "jobs": len(fallback), "path": "html"})
		return fallback, nil
	}

	if ferr != nil {
		if err != nil {
			return nil, fmt.Errorf("seek api fallback failed: %w (api error: %v)", ferr, err)
		}
		return nil, ferr
	}
	util.Debug("scraper_done", map[string]any{"site": s.SiteName(), "jobs": len(fallback), "path": "html"})
	return fallback, nil
}

func (s *Scraper) scrapeAPI(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	u, _ := url.Parse(s.apiURL)
	q := u.Query()
	if input.SearchTerm != "" {
		q.Set("keywords", input.SearchTerm)
	}
	if input.Location != "" {
		q.Set("where", input.Location)
	}
	q.Set("page", "1")
	u.RawQuery = q.Encode()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	req.Header.Set("Accept", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("seek request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("seek api unavailable status 404")
	}
	if resp.StatusCode == http.StatusForbidden {
		return nil, fmt.Errorf("seek api blocked status 403")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("seek status %d", resp.StatusCode)
	}

	var parsed struct {
		Data []struct {
			ID         string `json:"id"`
			Title      string `json:"title"`
			Teaser     string `json:"teaser"`
			Advertiser struct {
				Description string `json:"description"`
			} `json:"advertiser"`
			Location    string `json:"location"`
			ListingDate string `json:"listingDate"`
			JobURL      string `json:"jobUrl"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("seek decode: %w", err)
	}
	limit := input.ResultsWanted
	if limit <= 0 || limit > len(parsed.Data) {
		limit = len(parsed.Data)
	}
	jobs := make([]model.JobPost, 0, limit)
	for i := 0; i < limit; i++ {
		r := parsed.Data[i]
		var posted *time.Time
		if t, err := time.Parse(time.RFC3339, r.ListingDate); err == nil {
			posted = &t
		}
		jobs = append(jobs, model.JobPost{ID: "seek-" + strings.TrimSpace(r.ID), Title: strings.TrimSpace(r.Title), CompanyName: strings.TrimSpace(r.Advertiser.Description), JobURL: strings.TrimSpace(r.JobURL), Description: strings.TrimSpace(r.Teaser), Location: model.Location{City: strings.TrimSpace(r.Location)}, DatePosted: posted, IsRemote: strings.Contains(strings.ToLower(r.Location), "remote")})
	}
	return jobs, nil
}

func (s *Scraper) scrapeHTML(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	u, _ := url.Parse(s.listURL)
	q := u.Query()
	if strings.TrimSpace(input.SearchTerm) != "" {
		q.Set("keywords", strings.TrimSpace(input.SearchTerm))
	}
	if strings.TrimSpace(input.Location) != "" {
		q.Set("where", strings.TrimSpace(input.Location))
	}
	u.RawQuery = q.Encode()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("seek html request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusForbidden {
		return nil, fmt.Errorf("seek html blocked status 403")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("seek html status %d", resp.StatusCode)
	}
	b, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("seek html read: %w", err)
	}

	htmlBody := string(b)
	if strings.Contains(strings.ToLower(htmlBody), "just a moment") || strings.Contains(strings.ToLower(htmlBody), "security") {
		return nil, fmt.Errorf("seek html challenge page")
	}
	jobs := limitJobs(parseLDJSONJobs(htmlBody), input.ResultsWanted)
	if !util.HasMeaningfulJobs(jobs) {
		return nil, fmt.Errorf("seek no parseable jobs")
	}
	return jobs, nil
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
	out := make([]model.JobPost, 0)
	for i, s := range scripts {
		body := strings.TrimSpace(s[1])
		if body == "" {
			continue
		}
		var single ldJob
		if err := json.Unmarshal([]byte(body), &single); err == nil && strings.EqualFold(single.Type, "JobPosting") {
			out = append(out, toPost(single, i))
			continue
		}
		var arr []ldJob
		if err := json.Unmarshal([]byte(body), &arr); err == nil {
			for idx, j := range arr {
				if strings.EqualFold(j.Type, "JobPosting") {
					out = append(out, toPost(j, i*1000+idx))
				}
			}
			continue
		}
		var graph ldGraph
		if err := json.Unmarshal([]byte(body), &graph); err == nil {
			for idx, j := range graph.Graph {
				if strings.EqualFold(j.Type, "JobPosting") {
					out = append(out, toPost(j, i*1000+idx))
				}
			}
		}
	}
	return out
}

func toPost(j struct {
	Type        string `json:"@type"`
	Title       string `json:"title"`
	Description string `json:"description"`
	DatePosted  string `json:"datePosted"`
	URL         string `json:"url"`
	HiringOrg   struct {
		Name string `json:"name"`
	} `json:"hiringOrganization"`
}, seed int) model.JobPost {
	post := model.JobPost{
		ID:          fmt.Sprintf("seek-%s-%s-%d", util.NormalizeSlug(j.Title), util.NormalizeSlug(j.HiringOrg.Name), seed),
		Title:       strings.TrimSpace(j.Title),
		CompanyName: strings.TrimSpace(j.HiringOrg.Name),
		Description: strings.TrimSpace(j.Description),
		JobURL:      strings.TrimSpace(j.URL),
	}
	if post.JobURL == "" {
		post.JobURL = defaultList
	}
	post.DatePosted = util.ParseDatePosted(j.DatePosted)
	post.IsRemote = strings.Contains(strings.ToLower(post.Title+" "+post.Description), "remote")
	return post
}

func limitJobs(in []model.JobPost, wanted int) []model.JobPost {
	if wanted <= 0 || wanted > len(in) {
		return in
	}
	return in[:wanted]
}
