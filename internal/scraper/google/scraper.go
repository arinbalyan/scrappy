package google

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/arinbalyan/scrappy/internal/browser"
	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/util"
)

const searchURL = "https://www.google.com/search"

var (
	// titleCompanyRe matches Google's embedded job data in two formats:
	//   1. Legacy: ["Software Engineer","Company Name",...]
	//   2. Inline JS arrays in Google's data structures
	titleCompanyRe = regexp.MustCompile(`\[\s*"([^"]{5,120})"\s*,\s*"([^"]{2,100})"\s*,`)
	// jobURLRe extracts job application URLs from the page
	jobURLRe = regexp.MustCompile(`(https?://[^\s"\\<]+(?:careers|jobs|apply)[^\s"\\<]*)`)
	// afInitDataRe matches Google's AF_initDataCallback pattern which
	// contains job listing data in the current SERP format
	afInitDataRe = regexp.MustCompile(`AF_initDataCallback\s*\(\s*\{[^}]*data\s*:\s*(\[.*?\])\s*,\s*hash`)
)

type Scraper struct {
	client    *http.Client
	searchURL string
}

func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Retries: 3, Timeout: 25 * time.Second})
	}
	return &Scraper{client: client, searchURL: searchURL}
}

func NewWithURLs(client *http.Client, endpoint string) *Scraper {
	s := New(client)
	if strings.TrimSpace(endpoint) != "" {
		s.searchURL = strings.TrimSpace(endpoint)
	}
	return s
}

func (s *Scraper) SiteName() model.Site { return model.SiteGoogle }

func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	wanted := input.ResultsWanted
	if wanted <= 0 {
		wanted = 15
	}
	searchTerm := strings.TrimSpace(input.SearchTerm)
	if searchTerm == "" {
		return nil, fmt.Errorf("google_jobs: search term required")
	}

	query := buildQuery(searchTerm, input)
	jobs := make([]model.JobPost, 0, wanted)

	// ---------------------------------------------------------------
	// Primary: render in headless Chromium via Playwright.
	// Google serves JS-rendered content; HTTP-only paths rarely work.
	// ---------------------------------------------------------------
	if browser.IsAvailable() {
		u, _ := url.Parse(s.searchURL)
		q := u.Query()
		q.Set("q", buildQuery(input.SearchTerm, input))
		q.Set("udm", "8")
		q.Set("hl", "en")
		u.RawQuery = q.Encode()

		result, bErr := browser.FetchPage(ctx, u.String(), "")
		if bErr == nil && result.Status == 200 {
			page := util.ExtractJobPostingsJSONLD([]byte(result.HTML))
			for _, j := range page {
				jobs = append(jobs, j)
				if len(jobs) >= wanted {
					break
				}
			}
			if len(jobs) < wanted {
				page = parseJobs([]byte(result.HTML))
				for _, j := range page {
					jobs = append(jobs, j)
					if len(jobs) >= wanted {
						break
					}
				}
			}
		}
	}

	// ---------------------------------------------------------------
	// Fallback: try HTTP paths when browser isn't available.
	// ---------------------------------------------------------------
	if len(jobs) < wanted {
		// Try udm=8 SERP
		body, err := s.fetchPageStandard(ctx, query)
		if err == nil {
			page := util.ExtractJobPostingsJSONLD(body)
			for _, j := range page {
				jobs = append(jobs, j)
				if len(jobs) >= wanted {
					break
				}
			}
			if len(jobs) < wanted {
				page = parseJobs(body)
				for _, j := range page {
					jobs = append(jobs, j)
					if len(jobs) >= wanted {
						break
					}
				}
			}
			if len(jobs) < wanted {
				page = parseAFInitData(body)
				for _, j := range page {
					jobs = append(jobs, j)
					if len(jobs) >= wanted {
						break
					}
				}
			}
		}
	}

	if len(jobs) < wanted {
		// Try legacy ibp=htl;jobs endpoint
		bodyLegacy, legacyErr := s.fetchPage(ctx, query, 0)
		if legacyErr == nil {
			page := parseJobs(bodyLegacy)
			for _, j := range page {
				jobs = append(jobs, j)
				if len(jobs) >= wanted {
					break
				}
			}
			if len(jobs) < wanted {
				page = util.ExtractJobPostingsJSONLD(bodyLegacy)
				for _, j := range page {
					jobs = append(jobs, j)
					if len(jobs) >= wanted {
						break
					}
				}
			}
			if len(jobs) < wanted {
				page = parseAFInitData(bodyLegacy)
				for _, j := range page {
					jobs = append(jobs, j)
					if len(jobs) >= wanted {
						break
					}
				}
			}
		}
	}

	if !util.HasMeaningfulJobs(jobs) && len(jobs) == 0 {
		return nil, fmt.Errorf("google_jobs no parseable jobs")
	}
	if len(jobs) > wanted {
		jobs = jobs[:wanted]
	}
	return jobs, nil
}

func (s *Scraper) fetchPage(ctx context.Context, query string, start int) ([]byte, error) {
	u, _ := url.Parse(s.searchURL)
	q := u.Query()
	q.Set("q", query)
	q.Set("ibp", "htl;jobs")
	q.Set("hl", "en")
	if start > 0 {
		q.Set("start", fmt.Sprintf("%d", start))
		q.Set("asearch", "jbs")
		q.Set("async", "_id:VoQFxe,_pms:hts,_fmt:pc")
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("sec-fetch-dest", "document")
	req.Header.Set("sec-fetch-mode", "navigate")
	req.Header.Set("sec-fetch-site", "none")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("google_jobs request: %w", err)
	}
	defer resp.Body.Close()

	body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("google_jobs read: %w", err)
	}

	if challenge := util.DetectAntiBotChallenge(body); challenge != "" {
		return nil, fmt.Errorf("blocked - %s challenge detected", challenge)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("google_jobs status %d", resp.StatusCode)
	}

	return body, nil
}

// fetchPageStandard fetches a standard Google web SERP (udm=8) — this is
// the current default SERP format — and returns the body for JSON-LD parsing.
func (s *Scraper) fetchPageStandard(ctx context.Context, query string) ([]byte, error) {
	u, _ := url.Parse(s.searchURL)
	q := u.Query()
	q.Set("q", query)
	q.Set("udm", "8")
	q.Set("hl", "en")
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("sec-fetch-dest", "document")
	req.Header.Set("sec-fetch-mode", "navigate")
	req.Header.Set("sec-fetch-site", "none")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("google_jobs request: %w", err)
	}
	defer resp.Body.Close()

	body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("google_jobs read: %w", err)
	}

	if challenge := util.DetectAntiBotChallenge(body); challenge != "" {
		return nil, fmt.Errorf("blocked - %s challenge detected", challenge)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("google_jobs status %d", resp.StatusCode)
	}

	return body, nil
}

func buildQuery(term string, input model.ScraperInput) string {
	var parts []string
	parts = append(parts, term)
	if v := strings.TrimSpace(input.Location); v != "" {
		parts = append(parts, "near", v)
	}
	if input.IsRemote {
		parts = append(parts, "remote")
	}
	if v := strings.TrimSpace(string(input.JobType)); v != "" {
		parts = append(parts, v)
	}
	return strings.Join(parts, " ")
}

func parseJobs(raw []byte) []model.JobPost {
	html := string(raw)

	pairs := extractPairs(html)
	urls := jobURLRe.FindAllString(html, -1)

	jobs := make([]model.JobPost, 0, len(pairs))
	for i, p := range pairs {
		jobURL := ""
		if i < len(urls) {
			jobURL = urls[i]
		}
		if jobURL == "" {
			jobURL = fmt.Sprintf("https://www.google.com/search?q=%s", url.QueryEscape(p.title+" "+p.company+" jobs"))
		}
		jobs = append(jobs, model.JobPost{
			ID:          "go-" + util.HashID(jobURL),
			Title:       p.title,
			CompanyName: p.company,
			JobURL:      jobURL,
		})
	}

	return jobs
}

// parseAFInitData extracts job data from Google's AF_initDataCallback
// JavaScript callbacks embedded in the page HTML. Google uses these
// to pass structured data (including job listings) to the frontend.
func parseAFInitData(raw []byte) []model.JobPost {
	html := string(raw)

	// Try the full callback pattern first
	matches := afInitDataRe.FindStringSubmatch(html)
	if len(matches) >= 2 {
		dataStr := matches[1]
		// The data is a nested JS array; extract title/company pairs
		pairs := extractPairs(dataStr)
		jobs := make([]model.JobPost, 0, len(pairs))
		for _, p := range pairs {
			jobs = append(jobs, model.JobPost{
				ID:          "go-" + util.HashID(p.title+" "+p.company),
				Title:       p.title,
				CompanyName: p.company,
				JobURL:      fmt.Sprintf("https://www.google.com/search?q=%s", url.QueryEscape(p.title+" "+p.company+" jobs")),
			})
		}
		return jobs
	}

	// Try simpler patterns: look for inline JS arrays with job data
	return nil
}

type pair struct {
	title   string
	company string
}

func extractPairs(html string) []pair {
	matches := titleCompanyRe.FindAllStringSubmatch(html, -1)
	out := make([]pair, 0, len(matches))
	for _, m := range matches {
		if len(m) < 3 {
			continue
		}
		t := strings.TrimSpace(m[1])
		c := strings.TrimSpace(m[2])
		if t == "" || c == "" || strings.Contains(t, "http") || strings.HasPrefix(t, "function") {
			continue
		}
		out = append(out, pair{title: t, company: c})
	}
	return out
}


