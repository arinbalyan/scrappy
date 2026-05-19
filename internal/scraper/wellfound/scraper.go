package wellfound

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
)

const defaultURL = "https://wellfound.com/jobs"

var reCard = regexp.MustCompile(`(?s)<a[^>]*href="([^"]*/jobs/[^"]+)"[^>]*>.*?<h2[^>]*>([^<]+)</h2>.*?<span[^>]*class="company"[^>]*>([^<]+)</span>`)

type Scraper struct { client *http.Client; listURL string }
func New(client *http.Client) *Scraper { if client == nil { client = &http.Client{Timeout: 15 * time.Second} }; return &Scraper{client: client, listURL: defaultURL} }
func NewWithListURL(client *http.Client, u string) *Scraper { s := New(client); if strings.TrimSpace(u) != "" { s.listURL = u }; return s }
func (s *Scraper) SiteName() model.Site { return model.SiteWellfound }

func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, s.listURL, nil)
	resp, err := s.client.Do(req)
	if err != nil { return nil, fmt.Errorf("wellfound request: %w", err) }
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 { return nil, fmt.Errorf("wellfound status %d", resp.StatusCode) }
	b, _ := io.ReadAll(resp.Body)
	m := reCard.FindAllStringSubmatch(string(b), -1)
	limit := input.ResultsWanted
	if limit <= 0 || limit > len(m) { limit = len(m) }
	out := make([]model.JobPost, 0, limit)
	now := time.Now()
	for i := 0; i < limit; i++ {
		out = append(out, model.JobPost{ID: fmt.Sprintf("wf-%d", i+1), JobURL: m[i][1], Title: strings.TrimSpace(m[i][2]), CompanyName: strings.TrimSpace(m[i][3]), IsRemote: true, DatePosted: &now})
	}
	return out, nil
}
