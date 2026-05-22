package google

import (
	"context"
	"fmt"
	"hash/fnv"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/util"
)

const searchURL = "https://www.google.com/search"

var (
	titleCompanyRe = regexp.MustCompile(`\[\s*"([^"]{5,100})"\s*,\s*"([^"]{2,80})"\s*,`)
	jobURLRe       = regexp.MustCompile(`(https?://[^\s"\\]+(?:careers|jobs|apply)[^\s"\\]*)`)
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

	body, err := s.fetchPage(ctx, query, 0)
	if err != nil {
		return nil, fmt.Errorf("google_jobs initial: %w", err)
	}

	page := parseJobs(body)
	for _, j := range page {
		jobs = append(jobs, j)
		if len(jobs) >= wanted {
			break
		}
	}

	start := 10
	retries := 0
	maxRetries := 3
	for len(jobs) < wanted && retries < maxRetries {
		select {
		case <-ctx.Done():
			return jobs, ctx.Err()
		case <-time.After(3*time.Second + time.Duration(time.Now().UnixNano()%3)*time.Second):
		}

		body, err := s.fetchPage(ctx, query, start)
		if err != nil {
			retries++
			continue
		}

		page := parseJobs(body)
		if len(page) == 0 {
			break
		}
		for _, j := range page {
			jobs = append(jobs, j)
			if len(jobs) >= wanted {
				break
			}
		}
		start += 10
	}

	if !util.HasMeaningfulJobs(jobs) {
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

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("google_jobs request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("google_jobs status %d", resp.StatusCode)
	}

	body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("google_jobs read: %w", err)
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
			ID:          "go-" + hashID(jobURL),
			Title:       p.title,
			CompanyName: p.company,
			JobURL:      jobURL,
		})
	}

	return jobs
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
	if len(out) > 20 {
		out = out[:20]
	}
	return out
}

func hashID(s string) string {
	h := fnv.New64a()
	h.Write([]byte(s))
	return fmt.Sprintf("%d", h.Sum64())
}
