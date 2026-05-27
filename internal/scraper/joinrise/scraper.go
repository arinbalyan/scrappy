package joinrise

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/util"
)

const apiURL = "https://api.joinrise.ai/api/job/search"

// Scraper fetches jobs from the JoinRise API.
type Scraper struct {
	client *http.Client
	apiURL string
}

// New creates a new JoinRise scraper.
func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Retries: 2, Timeout: 20 * time.Second})
	}
	return &Scraper{client: client, apiURL: apiURL}
}

// NewWithAPIURL creates a scraper with a custom endpoint (used in tests).
func NewWithAPIURL(client *http.Client, endpoint string) *Scraper {
	s := New(client)
	if strings.TrimSpace(endpoint) != "" {
		s.apiURL = endpoint
	}
	return s
}

// SiteName returns the site identifier.
func (s *Scraper) SiteName() model.Site { return model.SiteJoinRise }

// Scrape fetches jobs from the JoinRise API.
func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{
		"site":           s.SiteName(),
		"results_wanted": input.ResultsWanted,
		"search_term":    input.SearchTerm,
		"location":       input.Location,
	})

	wanted := input.ResultsWanted
	if wanted <= 0 {
		wanted = 25
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("joinrise: build request: %w", err)
	}
	q := req.URL.Query()
	q.Set("page", "1")
	q.Set("limit", fmt.Sprintf("%d", wanted))
	q.Set("sort", "desc")
	q.Set("sortedBy", "createdAt")
	if strings.TrimSpace(input.Location) != "" {
		q.Set("jobLoc", strings.TrimSpace(input.Location))
	}
	req.URL.RawQuery = q.Encode()

	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("joinrise: request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("joinrise: status %d", resp.StatusCode)
	}

	body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("joinrise: read: %w", err)
	}

	var parsed apiResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("joinrise: decode: %w", err)
	}

	rawJobs := parsed.Result.Jobs
	if len(rawJobs) == 0 {
		return nil, fmt.Errorf("joinrise: no jobs returned")
	}

	term := strings.ToLower(strings.TrimSpace(input.SearchTerm))

	out := make([]model.JobPost, 0, wanted)
	for _, raw := range rawJobs {
		if len(out) >= wanted {
			break
		}
		title := strings.TrimSpace(raw.Title)
		jobURL := strings.TrimSpace(raw.URL)
		if title == "" || jobURL == "" {
			continue
		}

		// Client-side search term filtering
		if term != "" {
			summary := ""
			if raw.DescriptionBreakdown != nil {
				summary = strings.ToLower(raw.DescriptionBreakdown.OneSentenceSummary)
			}
			company := strings.ToLower(raw.Owner.CompanyName)
			hay := strings.ToLower(title) + " " + summary + " " + company
			for _, kw := range raw.DescriptionBreakdown.Keywords {
				hay += " " + strings.ToLower(kw)
			}
			if !strings.Contains(hay, term) {
				continue
			}
		}

		job := model.JobPost{
			ID:          "joinrise-" + strings.TrimSpace(raw.ID),
			Title:       title,
			CompanyName: strings.TrimSpace(raw.Owner.CompanyName),
			CompanyLogo: strings.TrimSpace(raw.Owner.Photo),
			JobURL:      jobURL,
			Site:        string(s.SiteName()),
		}

		// Description from one-sentence summary
		if raw.DescriptionBreakdown != nil {
			job.Description = strings.TrimSpace(raw.DescriptionBreakdown.OneSentenceSummary)
		}

		// Location
		if loc := strings.TrimSpace(raw.LocationAddress); loc != "" {
			job.Location.City = loc
		}

		// Remote detection
		rt := strings.ToLower(strings.TrimSpace(raw.Type))
		if rt == "remote" {
			job.IsRemote = true
		}
		if raw.DescriptionBreakdown != nil {
			wm := strings.ToLower(strings.TrimSpace(raw.DescriptionBreakdown.WorkModel))
			if wm == "remote" {
				job.IsRemote = true
			}
		}

		// Compensation
		if raw.DescriptionBreakdown != nil {
			comp := parseCompensation(raw.DescriptionBreakdown)
			if comp != nil {
				job.Compensation = comp
			}
		}

		// DatePosted
		if dp := strings.TrimSpace(raw.CreatedAt); dp != "" {
			job.DatePosted = parseDate(dp)
		}

		out = append(out, job)
	}

	util.Debug("scraper_done", map[string]any{
		"site": s.SiteName(),
		"jobs": len(out),
	})

	if !util.HasMeaningfulJobs(out) {
		return nil, fmt.Errorf("joinrise: no parseable jobs")
	}
	return out, nil
}

type apiResponse struct {
	Result struct {
		Jobs  []apiJob `json:"jobs"`
		Count int      `json:"count"`
	} `json:"result"`
}

type apiJob struct {
	ID                 string `json:"_id"`
	Title              string `json:"title"`
	URL                string `json:"url"`
	LocationAddress    string `json:"locationAddress"`
	Type               string `json:"type"`
	CreatedAt          string `json:"createdAt"`
	Owner              struct {
		CompanyName string `json:"companyName"`
		Photo       string `json:"photo"`
	} `json:"owner"`
	DescriptionBreakdown *descriptionBreakdown `json:"descriptionBreakdown"`
}

type descriptionBreakdown struct {
	OneSentenceSummary string   `json:"oneSentenceJobSummary"`
	SalaryRangeMin     *float64 `json:"salaryRangeMinYearly"`
	SalaryRangeMax     *float64 `json:"salaryRangeMaxYearly"`
	Keywords           []string `json:"keywords"`
	WorkModel          string   `json:"workModel"`
}

func parseCompensation(d *descriptionBreakdown) *model.Compensation {
	if d == nil {
		return nil
	}
	if d.SalaryRangeMin == nil && d.SalaryRangeMax == nil {
		return nil
	}
	minVal := 0.0
	maxVal := 0.0
	hasMin := false
	hasMax := false
	if d.SalaryRangeMin != nil {
		minVal = *d.SalaryRangeMin
		hasMin = true
	}
	if d.SalaryRangeMax != nil {
		maxVal = *d.SalaryRangeMax
		hasMax = true
	}
	if !hasMin && !hasMax {
		return nil
	}

	c := &model.Compensation{
		Interval: model.IntervalYearly,
		Currency: "USD",
	}
	if hasMin {
		c.MinAmount = &minVal
	}
	if hasMax {
		c.MaxAmount = &maxVal
	}
	return c
}

func parseDate(v string) *time.Time {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05",
		"2006-01-02",
	}
	for _, f := range formats {
		t, err := time.Parse(f, v)
		if err == nil {
			return &t
		}
	}
	return nil
}
