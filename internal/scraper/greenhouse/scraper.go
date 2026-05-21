package greenhouse

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

const boardFmt = "https://boards-api.greenhouse.io/v1/boards/%s/jobs"

type seedSource int

const (
	seedFromEnv seedSource = iota // SCRAPPY_GREENHOUSE_SEEDS
	seedFromSearch               // SearchTerm: company slug
	seedFromURL                  // SearchTerm: URL containing board slug
)

type Scraper struct{ Client *http.Client }

func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Retries: 3, Timeout: 18 * time.Second})
	}
	return &Scraper{Client: client}
}

// NewWithAPIURL creates a scraper that overrides the default board API URL.
func NewWithAPIURL(client *http.Client, apiURL string) *Scraper {
	s := New(client)
	_ = apiURL // kept for API symmetry
	return s
}

func (s *Scraper) SiteName() model.Site { return model.SiteGreenhouse }

func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{"site": s.SiteName(), "results_wanted": input.ResultsWanted, "search_term": input.SearchTerm, "location": input.Location})

	seeds, src := s.resolveSeeds(input)
	if len(seeds) == 0 {
		return nil, fmt.Errorf("greenhouse no seeds: set SCRAPPY_GREENHOUSE_SEEDS or pass a company name in --search (e.g. --search 'stripe' resolves to stripe's greenhouse board)")
	}
	util.Debug("greenhouse_seeds", map[string]any{"seeds": seeds, "src": src})

	out := make([]model.JobPost, 0, input.ResultsWanted)
	seen := map[string]struct{}{}
	for _, seed := range seeds {
		if input.ResultsWanted > 0 && len(out) >= input.ResultsWanted {
			break
		}
		jobs, err := s.fetchBoard(ctx, seed, input)
		if err != nil {
			util.Warn("greenhouse_seed_fail", map[string]any{"seed": seed, "err": err.Error()})
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
		return nil, fmt.Errorf("greenhouse no parseable jobs")
	}
	return out, nil
}

func (s *Scraper) resolveSeeds(input model.ScraperInput) ([]string, seedSource) {
	if seeds := parseSeeds(os.Getenv("SCRAPPY_GREENHOUSE_SEEDS")); len(seeds) > 0 {
		return seeds, seedFromEnv
	}

	raw := strings.TrimSpace(input.SearchTerm)
	if raw == "" {
		return nil, 0
	}

	if strings.Contains(raw, "greenhouse.io") {
		u, err := url.Parse(raw)
		if err == nil {
			host := u.Hostname()
			if strings.Contains(host, "greenhouse.io") {
				slug := extractSlugFromPath(u.Path)
				if slug != "" {
					return []string{slug}, seedFromURL
				}
			}
		}
	}

	slug := cleanGHSSlug(raw)
	if slug != "" {
		return []string{slug}, seedFromSearch
	}
	return nil, 0
}

func extractSlugFromPath(path string) string {
	path = strings.Trim(path, "/")
	parts := strings.Split(path, "/")
	for _, p := range parts {
		if p == "" || p == "v1" || p == "boards" || p == "jobs" {
			continue
		}
		return p
	}
	return ""
}

func cleanGHSSlug(raw string) string {
	if i := strings.Index(raw, "://"); i >= 0 {
		raw = raw[i+3:]
	}
	if i := strings.IndexAny(raw, "/?#"); i >= 0 {
		raw = raw[:i]
	}
	for _, suffix := range []string{".com", ".io", ".co", ".org", ".ai"} {
		if strings.HasSuffix(raw, suffix) {
			raw = strings.TrimSuffix(raw, suffix)
			break
		}
	}
	if i := strings.LastIndex(raw, "."); i >= 0 {
		raw = raw[i+1:]
	}
	raw = strings.ToLower(raw)
	var b strings.Builder
	for _, c := range raw {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' {
			b.WriteRune(c)
		}
	}
	slug := b.String()
	if len(slug) < 2 {
		return ""
	}
	return slug
}

func (s *Scraper) fetchBoard(ctx context.Context, boardToken string, input model.ScraperInput) ([]model.JobPost, error) {
	// Try with ?content=true for full descriptions
	type ghJob struct {
		ID          int    `json:"id"`
		Title       string `json:"title"`
		AbsoluteURL string `json:"absolute_url"`
		UpdatedAt   string `json:"updated_at"`
		LocationName string `json:"location_name_pretty"`
		Content     string `json:"content,omitempty"`
		Departments []struct {
			Name string `json:"name"`
		} `json:"Departments"`
	}
	type ghResponse struct {
		Jobs []ghJob `json:"jobs"`
	}

	u := fmt.Sprintf(boardFmt, boardToken) + "?content=true"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := s.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("greenhouse request %s: %w", boardToken, err)
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusTooManyRequests:
		return nil, fmt.Errorf("greenhouse rate limited 429 for %s", boardToken)
	case http.StatusForbidden, http.StatusUnauthorized:
		return nil, fmt.Errorf("greenhouse access denied %d for %s", resp.StatusCode, boardToken)
	case http.StatusNotFound:
		return nil, fmt.Errorf("greenhouse board not found 404 for %s", boardToken)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("greenhouse status %d for %s", resp.StatusCode, boardToken)
	}

	var parsed ghResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("greenhouse decode %s: %w", boardToken, err)
	}

	out := make([]model.JobPost, 0, len(parsed.Jobs))
	for _, r := range parsed.Jobs {
		if r.Title == "" {
			continue
		}
		jp := model.JobPost{
			ID:          fmt.Sprintf("gh-%s-%d", util.NormalizeSlug(boardToken), r.ID),
			Title:       r.Title,
			CompanyName: boardToken,
			JobURL:      r.AbsoluteURL,
			Location:    model.Location{City: r.LocationName},
			Description: r.Content,
		}
		if jp.JobURL == "" {
			jp.JobURL = fmt.Sprintf("https://boards.greenhouse.io/%s/jobs/%d", boardToken, r.ID)
		}
		if r.UpdatedAt != "" {
			if t, err := time.Parse(time.RFC3339, r.UpdatedAt); err == nil {
				jp.DatePosted = &t
			}
		}
		if len(r.Departments) > 0 {
			jp.Department = r.Departments[0].Name
		}
		out = append(out, jp)
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
