package lever

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/util"
)

const (
	postingsFmt = "https://api.lever.co/v0/postings/%s?mode=json"
	// Alternate path: https://jobs.lever.co/{slug}/ for posting websites
)

// seedSource indicates where the lever seeds come from
type seedSource int

const (
	seedFromEnv seedSource = iota // SCRAPPY_LEVER_SEEDS
	seedFromSearchTerm            // SearchTerm: company domain/slug
	seedFromJobsURL               // SearchTerm: https://company.jobs.lever.co
)

type Scraper struct{ Client *http.Client }

func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Retries: 3, Timeout: 18 * time.Second})
	}
	return &Scraper{Client: client}
}

// NewWithAPIURL creates a scraper that sends to `apiURL` instead of the default Lever API.
func NewWithAPIURL(client *http.Client, apiURL string) *Scraper {
	s := New(client)
	_ = apiURL // kept for API symmetry; not yet wired into full redirect-follow mode
	return s
}

func (s *Scraper) SiteName() model.Site { return model.SiteLever }

func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{"site": s.SiteName(), "results_wanted": input.ResultsWanted, "search_term": input.SearchTerm, "location": input.Location})

	seeds, src := s.resolveSeeds(input)
	if len(seeds) == 0 {
		return nil, fmt.Errorf("lever no seeds: set SCRAPPY_LEVER_SEEDS or pass a company domain/search term (e.g. --search 'stripe' resolves to api.lever.co/v0/postings/stripe)")
	}
	util.Debug("lever_seeds", map[string]any{"seeds": seeds, "src": src})

	out := make([]model.JobPost, 0, input.ResultsWanted)
	seen := map[string]struct{}{}
	for _, seed := range seeds {
		if input.ResultsWanted > 0 && len(out) >= input.ResultsWanted {
			break
		}
		jobs, err := s.fetchPostings(ctx, seed)
		if err != nil {
			util.Warn("lever_seed_fail", map[string]any{"seed": seed, "err": err.Error()})
			continue
		}
		for _, jp := range jobs {
			if _, ok := seen[jp.ID]; ok {
				continue
			}
			seen[jp.ID] = struct{}{}
			out = append(out, jp)
			if input.ResultsWanted > 0 && len(out) >= input.ResultsWanted {
				break
			}
		}
	}
	if !util.HasMeaningfulJobs(out) {
		return nil, fmt.Errorf("lever no parseable jobs")
	}
	return out, nil
}

func (s *Scraper) resolveSeeds(input model.ScraperInput) ([]string, seedSource) {
	if seeds := parseSeeds(os.Getenv("SCRAPPY_LEVER_SEEDS")); len(seeds) > 0 {
		return seeds, seedFromEnv
	}

	raw := strings.TrimSpace(input.SearchTerm)
	if raw == "" {
		return nil, 0
	}
	if strings.Contains(raw, "lever.co") {
		u, err := url.Parse(raw)
		if err == nil {
			host := u.Hostname()
			if parts := strings.SplitN(host, ".", 2); len(parts) > 0 {
				slug := parts[0]
				if slug != "jobs" && slug != "api" && slug != "www" {
					return []string{slug}, seedFromJobsURL
				}
			}
		}
	}
	slug := cleanLeverSlug(raw)
	if slug != "" {
		return []string{slug}, seedFromSearchTerm
	}
	return nil, 0
}

func cleanLeverSlug(raw string) string {
	raw = strings.ToLower(raw)
	raw = strings.TrimSpace(raw)

	// Strip scheme if present
	if i := strings.Index(raw, "://"); i >= 0 {
		raw = raw[i+3:]
	}
	if i := strings.IndexAny(raw, "/?#"); i >= 0 {
		raw = raw[:i]
	}

	// Extract last path component
	if i := strings.LastIndex(raw, "/"); i >= 0 {
		raw = raw[i+1:]
	}

	// Strip common TLDs
	raw = strings.TrimSuffix(raw, ".com")
	raw = strings.TrimSuffix(raw, ".io")
	raw = strings.TrimSuffix(raw, ".co")
	raw = strings.TrimSuffix(raw, ".org")
	raw = strings.TrimSuffix(raw, ".ai")

	// Remove non-alphanumeric chars
	var b strings.Builder
	for _, c := range raw {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') {
			b.WriteRune(c)
		} else if b.Len() > 0 {
			break
		}
	}
	slug := b.String()
	if len(slug) < 2 {
		return ""
	}
	return slug
}

func (s *Scraper) fetchPostings(ctx context.Context, seed string) ([]model.JobPost, error) {
	u := fmt.Sprintf(postingsFmt, seed)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := s.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("lever request %s: %w", seed, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, fmt.Errorf("lever rate limited 429 for %s", seed)
	}
	if resp.StatusCode == http.StatusForbidden {
		return nil, fmt.Errorf("lever access denied 403 for %s", seed)
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("lever site not found 404 for %s", seed)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("lever status %d for %s", resp.StatusCode, seed)
	}

	// Handle redirects for board URLs (jobs.lever.co/{slug}/)
	finalURL := resp.Request.URL
	if finalURL != nil {
		finalHost := finalURL.Hostname()
		if strings.Contains(finalHost, "lever.co") {
			if i := strings.Index(finalHost, ".lever.co"); i >= 0 {
				finalSlug := finalHost[:i]
				if finalSlug != "api" && finalSlug != "jobs" && finalSlug != "www" {
					seed = finalSlug
				}
			}
		}
	}

	var rows []struct {
		ID          string `json:"id"`
		Text        string `json:"text"`
		Description string `json:"description"`
		HostedUrl   string `json:"hostedUrl"`
		Categories  struct {
			Location string `json:"location"`
			Team     string `json:"team"`
		} `json:"categories"`
		CreatedAt int64  `json:"createdAt"`
		Company   string `json:"company"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
		return nil, fmt.Errorf("lever decode for %s: %w", seed, err)
	}

	out := make([]model.JobPost, 0, len(rows))
	for _, r := range rows {
		id := strings.TrimSpace(r.ID)
		if id == "" {
			continue
		}
		title := strings.TrimSpace(r.Text)
		company := strings.TrimSpace(r.Company)
		jobURL := strings.TrimSpace(r.HostedUrl)
		if jobURL == "" {
			jobURL = "https://jobs.lever.co/" + seed
		}
		out = append(out, model.JobPost{
			ID:          "lever-" + id,
			Title:       title,
			CompanyName: company,
			JobURL:      jobURL,
			Description: strings.TrimSpace(r.Description),
			Location:    model.Location{City: strings.TrimSpace(r.Categories.Location)},
			Department:  strings.TrimSpace(r.Categories.Team),
		})
		if r.CreatedAt > 0 {
			t := time.UnixMilli(r.CreatedAt)
			out[len(out)-1].DatePosted = &t
		}
		if strings.Contains(strings.ToLower(strings.TrimSpace(r.Categories.Location)), "remote") {
			out[len(out)-1].IsRemote = true
		}
	}
	return out, nil
}

func parseSeeds(v string) []string {
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
