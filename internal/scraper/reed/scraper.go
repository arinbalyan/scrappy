package reed

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/util"
)

const searchURL = "https://www.reed.co.uk/jobs"

// nextDataRe extracts the __NEXT_DATA__ JSON blob that Next.js embeds in the page.
// This is more reliable than HTML regex since reed.co.uk is a React SPA.
var nextDataRe = regexp.MustCompile(`<script id="__NEXT_DATA__" type="application/json">(?s)(.*?)</script>`)

const (
	salaryTypeDaily    = 2
	salaryTypeAnnual   = 5
	reedCurrency       = "GBP"
	maxRetries         = 3
	maxConsecutive429  = 3
	rateLimitDelayMin  = 200 * time.Millisecond
	rateLimitDelayMax  = 500 * time.Millisecond // 200-500ms jitter vs fixed 350ms
)

// Scraper scrapes Reed.co.uk job listings.
type Scraper struct {
	client    *http.Client
	searchURL string
}

// New creates a new Reed scraper with the given HTTP client.
func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Retries: 3, Timeout: 25 * time.Second})
	}
	return &Scraper{client: client, searchURL: searchURL}
}

// NewWithURLs creates a scraper with an overridable endpoint (used in tests).
func NewWithURLs(client *http.Client, endpoint string) *Scraper {
	s := New(client)
	if strings.TrimSpace(endpoint) != "" {
		s.searchURL = strings.TrimSpace(endpoint)
	}
	return s
}

// SiteName returns the site identifier.
func (s *Scraper) SiteName() model.Site { return model.SiteReed }

// Scrape fetches job listings from Reed.co.uk with pagination.
func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	wanted := input.ResultsWanted
	if wanted <= 0 {
		wanted = 15
	}
	util.Debug("scraper_reed_start", map[string]any{
		"search_term":    input.SearchTerm,
		"location":       input.Location,
		"results_wanted": wanted,
	})

	jobs := make([]model.JobPost, 0, wanted)
	pageno := 1
	consecutive429 := 0

	for len(jobs) < wanted {
		select {
		case <-ctx.Done():
			util.Debug("scraper_reed_cancelled", map[string]any{"jobs_found": len(jobs)})
			return jobs, ctx.Err()
		default:
		}

		body, err := s.fetchPage(ctx, input.SearchTerm, input.Location, pageno)
		if err != nil {
			if strings.Contains(err.Error(), "status 429") {
				consecutive429++
				util.Warn("reed_rate_limited", map[string]any{"consecutive": consecutive429, "page": pageno})
				if consecutive429 >= maxConsecutive429 {
					util.Warn("reed too many 429s, giving up", nil)
					break
				}
				exponentialBackoff(ctx, consecutive429)
				continue
			}
			util.Debug("reed non-429 error, stopping", map[string]any{"err": err.Error()})
			break
		}
		consecutive429 = 0

		page, err := parseJobs(body)
		if err != nil {
			util.Warn("scraper_reed_parse_error", map[string]any{"page": pageno, "err": err.Error()})
			break
		}
		if len(page) == 0 {
			util.Debug("scraper_reed_no_more", map[string]any{"page": pageno})
			break
		}

		for _, j := range page {
			jobs = append(jobs, j)
			if len(jobs) >= wanted {
				break
			}
		}

		pageno++
		// Rate limit: 200-500ms jittered delay
		if err := util.JitterSleep(ctx, rateLimitDelayMin, rateLimitDelayMax-rateLimitDelayMin); err != nil {
			return jobs, err
		}
	}

	if !util.HasMeaningfulJobs(jobs) {
		return nil, fmt.Errorf("reed no parseable jobs")
	}
	if len(jobs) > wanted {
		jobs = jobs[:wanted]
	}
	util.Debug("scraper_reed_done", map[string]any{"jobs": len(jobs)})
	return jobs, nil
}

// fetchPage downloads the HTML for a given search/page.
func (s *Scraper) fetchPage(ctx context.Context, searchTerm, location string, pageno int) ([]byte, error) {
	u, _ := url.Parse(s.searchURL)
	q := u.Query()
	if v := strings.TrimSpace(searchTerm); v != "" {
		q.Set("keywords", v)
	}
	if v := strings.TrimSpace(location); v != "" {
		q.Set("location", v)
	}
	if pageno > 1 {
		q.Set("pageno", fmt.Sprintf("%d", pageno))
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept-Language", "en-GB,en;q=0.9")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("reed request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("reed status %d", resp.StatusCode)
	}

	body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("reed read: %w", err)
	}
	return body, nil
}

// parseJobs extracts jobs from the __NEXT_DATA__ JSON embedded in the HTML.
func parseJobs(raw []byte) ([]model.JobPost, error) {
	m := nextDataRe.FindSubmatch(raw)
	if len(m) < 2 {
		return nil, fmt.Errorf("reed next_data script tag not found")
	}

	var nd nextData
	if err := json.Unmarshal(m[1], &nd); err != nil {
		return nil, fmt.Errorf("reed unmarshal next_data: %w", err)
	}

	if nd.Props.PageProps.SearchResults == nil {
		return nil, fmt.Errorf("reed search_results missing in next_data")
	}

	results := nd.Props.PageProps.SearchResults
	jobs := make([]model.JobPost, 0, len(results.Jobs))

	for _, rj := range results.Jobs {
		jd := rj.JobDetail
		if jd.JobTitle == "" || jd.JobID == 0 {
			continue
		}

		// --- Build compensation ---
		var comp *model.Compensation
		if jd.SalaryFrom > 0 || jd.SalaryTo > 0 {
			interval := model.IntervalYearly
			if jd.SalaryType == salaryTypeDaily {
				interval = model.IntervalDaily
			}
			minAmt := float64(jd.SalaryFrom)
			maxAmt := float64(jd.SalaryTo)
			comp = &model.Compensation{
				Interval:  interval,
				MinAmount: &minAmt,
				MaxAmount: &maxAmt,
				Currency:  reedCurrency,
			}
		}

		// --- Build full job URL ---
		jobURL := strings.TrimSpace(rj.URL)
		if jobURL != "" && !strings.HasPrefix(jobURL, "http") {
			jobURL = "https://www.reed.co.uk" + jobURL
		}

		// --- Parse date ---
		var datePosted *time.Time
		if jd.DisplayDate != "" {
			// Reed uses ISO8601-like formats: "2026-05-06T10:03:40.17"
			clean := jd.DisplayDate
			if len(clean) > 19 {
				clean = clean[:19]
			}
			if t, err := time.Parse("2006-01-02T15:04:05", clean); err == nil {
				datePosted = &t
			}
		}

		// --- Detect remote ---
		isRemote := false
		switch strings.ToLower(strings.TrimSpace(jd.RemoteWorkingOption)) {
		case "hybrid", "fullyremote":
			isRemote = true
		}

		// --- Company name ---
		companyName := jd.OuName
		if companyName == "" {
			companyName = rj.ProfileName
		}

		jobs = append(jobs, model.JobPost{
			ID:          fmt.Sprintf("reed-%d", jd.JobID),
			Title:       jd.JobTitle,
			CompanyName: companyName,
			JobURL:      jobURL,
			Location:    model.Location{City: jd.DisplayLocationName},
			Description: jd.JobDescription,
			Compensation: comp,
			DatePosted:  datePosted,
			IsRemote:    isRemote,
		})
	}

	return jobs, nil
}

// exponentialBackoff sleeps with exponential delay + jitter, respecting ctx cancellation.
func exponentialBackoff(ctx context.Context, retry int) {
	// Exponential: 500ms * 2^retry, capped at 4s
	base := time.Duration(500*(1<<retry)) * time.Millisecond
	if base > 4*time.Second {
		base = 4 * time.Second
	}
	jitter := time.Duration(rand.Intn(500)) * time.Millisecond
	wait := base + jitter
	_ = util.SleepWithContext(ctx, wait)
}

// --- JSON structures for __NEXT_DATA__ ---

type nextData struct {
	Props struct {
		PageProps struct {
			SearchResults *searchResults `json:"searchResults"`
		} `json:"pageProps"`
	} `json:"props"`
}

type searchResults struct {
	Count int      `json:"count"`
	Jobs  []rawJob `json:"jobs"`
}

type rawJob struct {
	JobDetail   jobDetail `json:"jobDetail"`
	URL         string    `json:"url"`
	ProfileName string    `json:"profileName"`
}

type jobDetail struct {
	JobID               int    `json:"jobId"`
	JobTitle            string `json:"jobTitle"`
	JobDescription      string `json:"jobDescription"`
	DisplayDate         string `json:"displayDate"`
	DisplayLocationName string `json:"displayLocationName"`
	SalaryFrom          int    `json:"salaryFrom"`
	SalaryTo            int    `json:"salaryTo"`
	SalaryType          int    `json:"salaryType"`
	SalaryDescription   int    `json:"salaryDescription"`
	OuName              string `json:"ouName"`
	JobType             int    `json:"jobType"`
	RemoteWorkingOption string `json:"remoteWorkingOption"`
}
