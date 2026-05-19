package google

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/util"
)

const defaultSearchURL = "https://www.google.com/search"
var reGoogleJob = regexp.MustCompile(`(?s)data-job-id="([^"]+)"[\s\S]*?<div[^>]*class="BjJfJf PUpOsf">([^<]+)</div>[\s\S]*?<div[^>]*class="Qk80Jf">([^<]+)</div>`)

type Scraper struct { client *http.Client; searchURL string }
func New(client *http.Client) *Scraper { if client == nil { client = util.NewHTTPClient(util.ClientOptions{Retries: 2, CookieResetEveryN: 100}) }; return &Scraper{client: client, searchURL: defaultSearchURL} }
func NewWithSearchURL(client *http.Client, u string) *Scraper { s := New(client); if strings.TrimSpace(u) != "" { s.searchURL = u }; return s }
func (s *Scraper) SiteName() model.Site { return model.SiteGoogle }

func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	u, _ := url.Parse(s.searchURL)
	q := u.Query()
	if input.GoogleSearchTerm != "" { q.Set("q", input.GoogleSearchTerm+" jobs") } else if input.SearchTerm != "" { q.Set("q", input.SearchTerm+" jobs") }
	u.RawQuery = q.Encode()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := s.client.Do(req)
	if err != nil { return nil, fmt.Errorf("google request: %w", err) }
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 { return nil, fmt.Errorf("google status %d", resp.StatusCode) }
	b, _ := io.ReadAll(resp.Body)
	m := reGoogleJob.FindAllStringSubmatch(string(b), -1)
	limit := input.ResultsWanted
	if limit <= 0 || limit > len(m) { limit = len(m) }
	out := make([]model.JobPost, 0, limit)
	for i := 0; i < limit; i++ { out = append(out, model.JobPost{ID: "go-" + m[i][1], Title: strings.TrimSpace(m[i][2]), CompanyName: strings.TrimSpace(m[i][3]), JobURL: "https://www.google.com/search?q=" + url.QueryEscape(m[i][2])}) }
	return out, nil
}
