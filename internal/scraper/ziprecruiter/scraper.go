package ziprecruiter

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/arinbalyan/scrappy/internal/model"
)

const defaultURL = "https://www.ziprecruiter.com/jobs-search"
var reJob = regexp.MustCompile(`(?s)data-job-id="([^"]+)"[\s\S]*?<a[^>]*class="job_content"[^>]*href="([^"]+)"[\s\S]*?<h2[^>]*>([^<]+)</h2>[\s\S]*?<a[^>]*class="t_org_link"[^>]*>([^<]+)</a>`)

type Scraper struct { client *http.Client; listURL string }
func New(client *http.Client) *Scraper { if client == nil { client = &http.Client{} }; return &Scraper{client: client, listURL: defaultURL} }
func NewWithListURL(client *http.Client, u string) *Scraper { s := New(client); if strings.TrimSpace(u) != "" { s.listURL = u }; return s }
func (s *Scraper) SiteName() model.Site { return model.SiteZipRecruiter }

func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, s.listURL, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := s.client.Do(req)
	if err != nil { return nil, fmt.Errorf("ziprecruiter request: %w", err) }
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 { return nil, fmt.Errorf("ziprecruiter status %d", resp.StatusCode) }
	b, _ := io.ReadAll(resp.Body)
	m := reJob.FindAllStringSubmatch(string(b), -1)
	limit := input.ResultsWanted
	if limit <= 0 || limit > len(m) { limit = len(m) }
	out := make([]model.JobPost, 0, limit)
	for i := 0; i < limit; i++ { out = append(out, model.JobPost{ID: "zr-" + m[i][1], JobURL: m[i][2], Title: strings.TrimSpace(m[i][3]), CompanyName: strings.TrimSpace(m[i][4])}) }
	return out, nil
}
