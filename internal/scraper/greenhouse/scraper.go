package greenhouse

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/scraper/ats"
	"github.com/arinbalyan/scrappy/internal/util"
)

const boardFmt = "https://boards-api.greenhouse.io/v1/boards/%s/jobs"

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

	// Use the shared ATS seed resolution which checks env var → config/company_slugs.toml.
	// The config has 145+ greenhouse company seeds (stripe, airbnb, lyft, ...).
	// We explicitly reject SeedFromSearch to prevent search terms like
	// "AI Engineer OR ML Engineer" from being used as board slugs.
	seeds, src, _ := ats.ResolveSeedsWithMeta(input.SearchTerm, "SCRAPPY_GREENHOUSE_SEEDS")
	if src == ats.SeedFromSearch {
		seeds = nil
	}
	if len(seeds) == 0 {
		util.Debug("greenhouse_skip", map[string]any{"reason": "no seeds configured — set SCRAPPY_GREENHOUSE_SEEDS or add greenhouse: slugs to config/company_slugs.toml"})
		return nil, fmt.Errorf("greenhouse no seeds: set SCRAPPY_GREENHOUSE_SEEDS env var (comma-separated company slugs) or add entries to config/company_slugs.toml")
	}
	util.Debug("greenhouse_seeds", map[string]any{"seeds": seeds, "src": ats.SeedSourceString(src)})

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

func (s *Scraper) fetchBoard(ctx context.Context, boardToken string, input model.ScraperInput) ([]model.JobPost, error) {
	// Try with ?content=true for full descriptions
	type ghJob struct {
		ID          int    `json:"id"`
		Title       string `json:"title"`
		AbsoluteURL string `json:"absolute_url"`
		UpdatedAt   string `json:"updated_at"`
		Location    *struct {
			Name string `json:"name"`
		} `json:"location"`
		Content     string `json:"content,omitempty"`
		Department string `json:"departments,omitempty"`
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
		loc := model.Location{}
		if r.Location != nil && strings.TrimSpace(r.Location.Name) != "" {
			loc = parseLocation(r.Location.Name)
		}
		jp := model.JobPost{
			ID:          fmt.Sprintf("gh-%s-%d", util.NormalizeSlug(boardToken), r.ID),
			Title:       r.Title,
			CompanyName: boardToken,
			JobURL:      r.AbsoluteURL,
			Location:    loc,
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
		if r.Department != "" {
			jp.Department = r.Department
		}
		out = append(out, jp)
	}
	return out, nil
}

// parseLocation splits "City, State Country" into City and State fields.
func parseLocation(v string) model.Location {
	parts := strings.SplitN(v, ", ", 2)
	loc := model.Location{}
	if len(parts) > 0 {
		loc.City = strings.TrimSpace(parts[0])
	}
	if len(parts) > 1 {
		rest := strings.TrimSpace(parts[1])
		// If it looks like "CA" or "California", treat as State
		if !strings.ContainsAny(rest, " ") || len(rest) <= 3 {
			loc.State = rest
		} else {
			loc.City = v // keep original as city if it's complex
		}
	}
	return loc
}


