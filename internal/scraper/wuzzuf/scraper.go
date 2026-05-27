package wuzzuf

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/util"
)

const defaultBaseURL = "https://wuzzuf.net"

var (
	jobURLRe = regexp.MustCompile(`href=["'](/jobs/p/[^"'#?]+)["']`)
	titleRe  = regexp.MustCompile(`(?i)<a[^>]*href=["']/jobs/p/[^"'#?]+["'][^>]*>([^<]+)</a>`)
)

type Scraper struct {
	client  *http.Client
	baseURL string
}

func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Timeout: 20 * time.Second})
	}
	return &Scraper{client: client, baseURL: defaultBaseURL}
}

func NewWithBaseURL(client *http.Client, baseURL string) *Scraper {
	s := New(client)
	if strings.TrimSpace(baseURL) != "" {
		s.baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	}
	return s
}

func (s *Scraper) SiteName() model.Site { return model.SiteWuzzuf }

func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	wanted := input.ResultsWanted
	if wanted <= 0 {
		wanted = 25
	}

	term := strings.TrimSpace(input.SearchTerm)
	if term == "" {
		term = "software"
	}

	u, err := url.Parse(s.baseURL + "/search/jobs/")
	if err != nil {
		return nil, fmt.Errorf("wuzzuf: build url: %w", err)
	}
	q := u.Query()
	q.Set("q", term)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("wuzzuf: build request: %w", err)
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("wuzzuf: request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("wuzzuf: status %d", resp.StatusCode)
	}

	body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("wuzzuf: read: %w", err)
	}

	html := string(body)
	matches := jobURLRe.FindAllStringSubmatch(html, -1)
	if len(matches) == 0 {
		return nil, fmt.Errorf("wuzzuf: no parseable jobs")
	}

	titleMatches := titleRe.FindAllStringSubmatch(html, -1)
	titles := make([]string, 0, len(titleMatches))
	for _, m := range titleMatches {
		if len(m) < 2 {
			continue
		}
		t := strings.TrimSpace(util.StripHTML(m[1]))
		if t != "" {
			titles = append(titles, t)
		}
	}

	out := make([]model.JobPost, 0, wanted)
	seen := map[string]struct{}{}
	for i, m := range matches {
		if len(out) >= wanted {
			break
		}
		if len(m) < 2 {
			continue
		}
		path := strings.TrimSpace(m[1])
		if path == "" {
			continue
		}
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}

		title := "Wuzzuf Job"
		if i < len(titles) && titles[i] != "" {
			title = titles[i]
		}

		jobURL := s.baseURL + path
		out = append(out, model.JobPost{
			ID:          "wz-" + util.HashID(path),
			Title:       title,
			CompanyName: "Unknown Employer",
			JobURL:      jobURL,
			Site:        string(model.SiteWuzzuf),
		})
	}

	if !util.HasMeaningfulJobs(out) {
		return nil, fmt.Errorf("wuzzuf: no parseable jobs")
	}
	return out, nil
}
