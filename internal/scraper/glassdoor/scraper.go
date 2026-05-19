package glassdoor

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/arinbalyan/scrappy/internal/model"
)

const defaultURL = "https://www.glassdoor.com/Job/jobs.htm"
var reJob = regexp.MustCompile(`(?s)data-jobid="([^"]+)"[\s\S]*?<a[^>]*class="jobLink"[^>]*>([^<]+)</a>[\s\S]*?<span[^>]*class="EmployerProfile_compactEmployerName"[^>]*>([^<]+)</span>`)

type Scraper struct { client *http.Client; listURL string }
func New(client *http.Client) *Scraper { if client == nil { client = &http.Client{} }; return &Scraper{client: client, listURL: defaultURL} }
func NewWithListURL(client *http.Client, u string) *Scraper { s := New(client); if strings.TrimSpace(u) != "" { s.listURL = u }; return s }
func (s *Scraper) SiteName() model.Site { return model.SiteGlassdoor }

func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, s.listURL, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := s.client.Do(req)
	if err != nil { return nil, fmt.Errorf("glassdoor request: %w", err) }
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 { return nil, fmt.Errorf("glassdoor status %d", resp.StatusCode) }
	b, _ := io.ReadAll(resp.Body)
	m := reJob.FindAllStringSubmatch(string(b), -1)
	limit := input.ResultsWanted
	if limit <= 0 || limit > len(m) { limit = len(m) }
	out := make([]model.JobPost, 0, limit)
	for i := 0; i < limit; i++ { out = append(out, model.JobPost{ID: "gd-" + m[i][1], Title: strings.TrimSpace(m[i][2]), CompanyName: strings.TrimSpace(m[i][3]), JobURL: s.listURL}) }
	return out, nil
}
