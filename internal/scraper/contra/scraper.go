package contra

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/util"
)

const defaultURL = "https://contra.com/opportunities"

var reJob = regexp.MustCompile(`(?is)<a[^>]*href="([^"]+)"[^>]*>([^<]{4,140})</a>`)

type Scraper struct {
	client  *http.Client
	listURL string
}

func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Retries: 3, Timeout: 18 * time.Second})
	}
	return &Scraper{client: client, listURL: defaultURL}
}

func NewWithListURL(client *http.Client, u string) *Scraper {
	s := New(client)
	if strings.TrimSpace(u) != "" {
		s.listURL = strings.TrimSpace(u)
	}
	return s
}

func (s *Scraper) SiteName() model.Site { return model.SiteContra }

func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, s.listURL, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("contra request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("contra status %d", resp.StatusCode)
	}
	b, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("contra read: %w", err)
	}
	m := reJob.FindAllStringSubmatch(string(b), -1)
	out := make([]model.JobPost, 0, len(m))
	seen := map[string]struct{}{}
	for i, row := range m {
		u := strings.TrimSpace(row[1])
		if strings.HasPrefix(u, "/") {
			u = strings.TrimRight(s.listURL, "/") + u
		}
		if !strings.HasPrefix(u, "http") {
			continue
		}
		if _, ok := seen[u]; ok {
			continue
		}
		seen[u] = struct{}{}
		title := strings.TrimSpace(row[2])
		if title == "" {
			continue
		}
		out = append(out, model.JobPost{ID: fmt.Sprintf("contra-%d", i+1), Title: title, JobURL: u, IsRemote: true})
		if input.ResultsWanted > 0 && len(out) >= input.ResultsWanted {
			break
		}
	}
	if !util.HasMeaningfulJobs(out) {
		return nil, fmt.Errorf("contra no parseable jobs")
	}
	return out, nil
}
