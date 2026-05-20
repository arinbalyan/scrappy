package naukri

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/util"
)

const defaultAPI = "https://www.naukri.com/jobapi/v3/search"

type Scraper struct {
	client *http.Client
	apiURL string
}

func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Retries: 3, CookieResetEveryN: 150, Timeout: 18 * time.Second})
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

func (s *Scraper) SiteName() model.Site { return model.SiteNaukri }

func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{"site": s.SiteName(), "results_wanted": input.ResultsWanted, "search_term": input.SearchTerm, "location": input.Location})

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
	util.Debug("scraper_done", map[string]any{"site": s.SiteName(), "jobs": len(jobs)})
	return jobs, nil
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
