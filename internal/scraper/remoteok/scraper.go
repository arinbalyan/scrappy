package remoteok

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

const defaultAPI = "https://remoteok.com/api"

type Scraper struct {
	client *http.Client
	apiURL string
}

func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Retries: 2, CookieResetEveryN: 120, Timeout: 15 * time.Second})
	}
	return &Scraper{client: client, apiURL: defaultAPI}
}
func NewWithAPIURL(client *http.Client, endpoint string) *Scraper {
	s := New(client)
	if strings.TrimSpace(endpoint) != "" {
		s.apiURL = endpoint
	}
	return s
}
func (s *Scraper) SiteName() model.Site { return model.SiteRemoteOK }

func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	util.Debug("scraper_start", map[string]any{"site": s.SiteName(), "results_wanted": input.ResultsWanted, "search_term": input.SearchTerm, "location": input.Location})

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, s.apiURL, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("remoteok request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("remoteok status %d", resp.StatusCode)
	}

	var raw []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("remoteok decode: %w", err)
	}
	if len(raw) <= 1 {
		return nil, fmt.Errorf("remoteok: empty or metadata-only response")
	}
	raw = raw[1:]

	limit := input.ResultsWanted
	if limit <= 0 || limit > len(raw) {
		limit = len(raw)
	}
	out := make([]model.JobPost, 0, limit)
	for i := 0; i < limit; i++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		j := raw[i]
		title, _ := j["position"].(string)
		company, _ := j["company"].(string)
		url, _ := j["url"].(string)
		id := fmt.Sprintf("rok-%v", j["id"])
		var posted *time.Time
		if epoch, ok := j["epoch"].(float64); ok {
			t := time.Unix(int64(epoch), 0)
			posted = &t
		}
		out = append(out, model.JobPost{ID: id, Title: title, CompanyName: company, JobURL: url, IsRemote: true, DatePosted: posted})
	}
	util.Debug("scraper_done", map[string]any{"site": s.SiteName(), "jobs": len(out)})
	if !util.HasMeaningfulJobs(out) {
		return nil, fmt.Errorf("remoteok no parseable jobs")
	}
	return out, nil
}
