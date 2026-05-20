package naukri

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/util"
)

const (
	defaultAPI  = "https://www.naukri.com/jobapi/v3/search"
	defaultList = "https://www.naukri.com/jobs"
)

var reNaukriLD = regexp.MustCompile(`(?s)<script[^>]*type="application/ld\+json"[^>]*>(.*?)</script>`)

type Scraper struct {
	client  *http.Client
	apiURL  string
	listURL string
}

func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Retries: 3, CookieResetEveryN: 150, Timeout: 18 * time.Second})
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

func (s *Scraper) SiteName() model.Site { return model.SiteNaukri }

func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{"site": s.SiteName(), "results_wanted": input.ResultsWanted, "search_term": input.SearchTerm, "location": input.Location})

	jobs, err := s.scrapeAPI(ctx, input)
	if err == nil && util.HasMeaningfulJobs(jobs) {
		util.Debug("scraper_done", map[string]any{"site": s.SiteName(), "jobs": len(jobs), "path": "api"})
		return jobs, nil
	}

	fallback, ferr := s.scrapeHTML(ctx, input)
	if ferr != nil {
		if err != nil {
			return nil, fmt.Errorf("naukri api fallback failed: %w (api error: %v)", ferr, err)
		}
		return nil, ferr
	}
	util.Debug("scraper_done", map[string]any{"site": s.SiteName(), "jobs": len(fallback), "path": "html"})
	return fallback, nil
}

func (s *Scraper) scrapeAPI(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	u, _ := url.Parse(s.apiURL)
	q := u.Query()
	if strings.TrimSpace(input.SearchTerm) != "" {
		q.Set("q", strings.TrimSpace(input.SearchTerm))
	}
	if strings.TrimSpace(input.Location) != "" {
		q.Set("l", strings.TrimSpace(input.Location))
	}
	q.Set("noOfResults", strconv.Itoa(pageSize(input.ResultsWanted)))
	u.RawQuery = q.Encode()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("naukri request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("naukri status %d", resp.StatusCode)
	}

	var parsed struct {
		JobDetails []struct {
			JobID             string   `json:"jobId"`
			Title             string   `json:"title"`
			CompanyName       string   `json:"companyName"`
			Placeholders      []string `json:"placeholders"`
			JobDescription    string   `json:"jobDescription"`
			JobURL            string   `json:"jdURL"`
			TagsAndSkills     []string `json:"tagsAndSkills"`
			ExperienceText    string   `json:"experienceText"`
			CompanyRating     string   `json:"companyRating"`
			ReviewCount       string   `json:"reviewCount"`
			VacancyCount      string   `json:"vacancyCount"`
			WfhType           string   `json:"wfhType"`
			FooterPlaceholder string   `json:"footerPlaceholderLabel"`
		} `json:"jobDetails"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("naukri decode: %w", err)
	}

	limit := input.ResultsWanted
	if limit <= 0 || limit > len(parsed.JobDetails) {
		limit = len(parsed.JobDetails)
	}
	jobs := make([]model.JobPost, 0, limit)
	for i := 0; i < limit; i++ {
		r := parsed.JobDetails[i]
		loc := ""
		if len(r.Placeholders) > 0 {
			loc = r.Placeholders[0]
		}
		rating := parseOptionalFloat(r.CompanyRating)
		reviews := parseOptionalInt(r.ReviewCount)
		vacancy := parseOptionalInt(r.VacancyCount)
		jobs = append(jobs, model.JobPost{
			ID:              "naukri-" + strings.TrimSpace(r.JobID),
			Title:           strings.TrimSpace(r.Title),
			CompanyName:     strings.TrimSpace(r.CompanyName),
			Location:        model.Location{City: strings.TrimSpace(loc)},
			JobURL:          strings.TrimSpace(r.JobURL),
			Description:     strings.TrimSpace(r.JobDescription),
			IsRemote:        strings.Contains(strings.ToLower(r.WfhType+" "+r.FooterPlaceholder), "remote"),
			Skills:          compactStrings(r.TagsAndSkills),
			ExperienceRange: strings.TrimSpace(r.ExperienceText),
			CompanyRating:   rating,
			CompanyReviews:  reviews,
			VacancyCount:    vacancy,
			WorkFromHome:    strings.TrimSpace(r.WfhType),
		})
	}
	return jobs, nil
}

func (s *Scraper) scrapeHTML(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	u, _ := url.Parse(s.listURL)
	q := u.Query()
	if strings.TrimSpace(input.SearchTerm) != "" {
		q.Set("k", strings.TrimSpace(input.SearchTerm))
	}
	if strings.TrimSpace(input.Location) != "" {
		q.Set("l", strings.TrimSpace(input.Location))
	}
	u.RawQuery = q.Encode()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("naukri html request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("naukri html status %d", resp.StatusCode)
	}
	b, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("naukri html read: %w", err)
	}
	jobs := parseNaukriLD(string(b))
	jobs = limitNaukriJobs(jobs, input.ResultsWanted)
	if !util.HasMeaningfulJobs(jobs) {
		return nil, nil
	}
	return jobs, nil
}

func parseNaukriLD(raw string) []model.JobPost {
	type ldJob struct {
		Type        string `json:"@type"`
		Title       string `json:"title"`
		Description string `json:"description"`
		DatePosted  string `json:"datePosted"`
		URL         string `json:"url"`
		JobLocation any    `json:"jobLocation"`
		HiringOrg   struct {
			Name string `json:"name"`
		} `json:"hiringOrganization"`
	}
	type ldGraph struct {
		Graph []ldJob `json:"@graph"`
	}

	scripts := reNaukriLD.FindAllStringSubmatch(raw, -1)
	out := make([]model.JobPost, 0)
	for i, s := range scripts {
		body := strings.TrimSpace(s[1])
		if body == "" {
			continue
		}
		var single ldJob
		if err := json.Unmarshal([]byte(body), &single); err == nil && strings.EqualFold(single.Type, "JobPosting") {
			out = append(out, toLDPost(single, i))
			continue
		}
		var arr []ldJob
		if err := json.Unmarshal([]byte(body), &arr); err == nil {
			for idx, j := range arr {
				if strings.EqualFold(j.Type, "JobPosting") {
					out = append(out, toLDPost(j, i*1000+idx))
				}
			}
			continue
		}
		var graph ldGraph
		if err := json.Unmarshal([]byte(body), &graph); err == nil {
			for idx, j := range graph.Graph {
				if strings.EqualFold(j.Type, "JobPosting") {
					out = append(out, toLDPost(j, i*1000+idx))
				}
			}
		}
	}
	return out
}

func toLDPost(j struct {
	Type        string `json:"@type"`
	Title       string `json:"title"`
	Description string `json:"description"`
	DatePosted  string `json:"datePosted"`
	URL         string `json:"url"`
	JobLocation any    `json:"jobLocation"`
	HiringOrg   struct {
		Name string `json:"name"`
	} `json:"hiringOrganization"`
}, seed int) model.JobPost {
	post := model.JobPost{
		ID:          fmt.Sprintf("naukri-%s-%s-%d", util.NormalizeSlug(j.Title), util.NormalizeSlug(j.HiringOrg.Name), seed),
		Title:       strings.TrimSpace(j.Title),
		CompanyName: strings.TrimSpace(j.HiringOrg.Name),
		Description: strings.TrimSpace(j.Description),
		JobURL:      strings.TrimSpace(j.URL),
		DatePosted:  util.ParseDatePosted(j.DatePosted),
	}
	if post.JobURL == "" {
		post.JobURL = defaultList
	}
	post.IsRemote = strings.Contains(strings.ToLower(post.Title+" "+post.Description), "remote")
	return post
}

func limitNaukriJobs(in []model.JobPost, wanted int) []model.JobPost {
	if wanted <= 0 || wanted > len(in) {
		return in
	}
	return in[:wanted]
}

func pageSize(resultsWanted int) int {
	if resultsWanted <= 0 {
		return 20
	}
	if resultsWanted > 50 {
		return 50
	}
	return resultsWanted
}

func compactStrings(v []string) []string {
	out := make([]string, 0, len(v))
	for _, s := range v {
		s = strings.TrimSpace(s)
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

func parseOptionalFloat(v string) *float64 {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return nil
	}
	return &f
}

func parseOptionalInt(v string) int {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0
	}
	return n
}
