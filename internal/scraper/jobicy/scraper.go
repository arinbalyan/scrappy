package jobicy

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

const apiURL = "https://jobicy.com/api/v2/remote-jobs"

type apiResponse struct {
	Jobs []apiJob `json:"jobs"`
}

type apiJob struct {
	ID              int      `json:"id"`
	URL             string   `json:"url"`
	JobTitle        string   `json:"jobTitle"`
	CompanyName     string   `json:"companyName"`
	JobIndustry     []string `json:"jobIndustry"`
	JobGeo          string   `json:"jobGeo"`
	JobLevel        string   `json:"jobLevel"`
	JobDescription  string   `json:"jobDescription"`
	PubDate         string   `json:"pubDate"`
	AnnualSalaryMin *float64 `json:"annualSalaryMin"`
	AnnualSalaryMax *float64 `json:"annualSalaryMax"`
	SalaryCurrency  string   `json:"salaryCurrency"`
}

type Scraper struct {
	client *http.Client
	apiURL string
}

func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Retries: 2, Timeout: 15 * time.Second})
	}
	return &Scraper{client: client, apiURL: apiURL}
}

func NewWithAPIURL(client *http.Client, endpoint string) *Scraper {
	s := New(client)
	if strings.TrimSpace(endpoint) != "" {
		s.apiURL = strings.TrimSpace(endpoint)
	}
	return s
}

func (s *Scraper) SiteName() model.Site { return model.SiteJobicy }

func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	wanted := input.ResultsWanted
	if wanted <= 0 {
		wanted = 25
	}
	if wanted > 50 {
		wanted = 50
	}

	u, err := url.Parse(s.apiURL)
	if err != nil {
		return nil, fmt.Errorf("jobicy parse url: %w", err)
	}
	q := u.Query()
	q.Set("count", strconv.Itoa(wanted))
	u.RawQuery = q.Encode()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("jobicy request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("jobicy status %d", resp.StatusCode)
	}

	body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("jobicy read: %w", err)
	}

	var parsed apiResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("jobicy decode: %w", err)
	}

	term := strings.ToLower(strings.TrimSpace(input.SearchTerm))
	jobs := make([]model.JobPost, 0, len(parsed.Jobs))
	for _, r := range parsed.Jobs {
		if len(jobs) >= wanted {
			break
		}
		if strings.TrimSpace(r.JobTitle) == "" || strings.TrimSpace(r.URL) == "" {
			continue
		}
		if term != "" {
			hay := strings.ToLower(r.JobTitle + " " + r.JobDescription + " " + strings.Join(r.JobIndustry, " "))
			if !strings.Contains(hay, term) {
				continue
			}
		}

		job := model.JobPost{
			ID:          fmt.Sprintf("jobicy-%d", r.ID),
			Title:       strings.TrimSpace(r.JobTitle),
			CompanyName: strings.TrimSpace(r.CompanyName),
			JobURL:      strings.TrimSpace(r.URL),
			Description: r.JobDescription,
			IsRemote:    true,
			Location: model.Location{
				City: strings.TrimSpace(r.JobGeo),
			},
			JobLevel:        strings.TrimSpace(r.JobLevel),
			CompanyIndustry: strings.Join(r.JobIndustry, ", "),
		}
		if r.PubDate != "" {
			if t, err := time.Parse(time.RFC3339, r.PubDate); err == nil {
				job.DatePosted = &t
			}
		}
		if r.AnnualSalaryMin != nil || r.AnnualSalaryMax != nil {
			job.Compensation = &model.Compensation{
				Interval:  model.IntervalYearly,
				MinAmount: r.AnnualSalaryMin,
				MaxAmount: r.AnnualSalaryMax,
				Currency:  strings.TrimSpace(r.SalaryCurrency),
			}
		}
		jobs = append(jobs, job)
	}

	if !util.HasMeaningfulJobs(jobs) {
		return nil, fmt.Errorf("jobicy no parseable jobs")
	}
	return jobs, nil
}
