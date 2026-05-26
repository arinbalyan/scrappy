package navjobs

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

const feedURL = "https://jobinformasjon.nav.no/api/ledige-stillinger/feed"
const tokenURL = "https://jobinformasjon.nav.no/api/ledige-stillinger/token"

// Scraper fetches jobs from the NAV Jobs feed.
type Scraper struct {
	client   *http.Client
	feedURL  string
	tokenURL string
	token    string
}

// New creates a new NAV Jobs scraper.
func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Retries: 2, Timeout: 20 * time.Second})
	}
	return &Scraper{client: client, feedURL: feedURL, tokenURL: tokenURL}
}

// NewWithFeedURL creates a scraper with a custom endpoint (used in tests).
func NewWithFeedURL(client *http.Client, feedEndpoint string) *Scraper {
	s := New(client)
	if strings.TrimSpace(feedEndpoint) != "" {
		s.feedURL = feedEndpoint
	}
	return s
}

// NewWithToken creates a scraper with a pre-configured token.
func NewWithToken(client *http.Client, token string) *Scraper {
	s := New(client)
	if strings.TrimSpace(token) != "" {
		s.token = token
	}
	return s
}

// SiteName returns the site identifier.
func (s *Scraper) SiteName() model.Site { return model.SiteNavJobs }

// Scrape fetches jobs from the NAV Jobs feed using a bearer token.
func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{
		"site":           s.SiteName(),
		"results_wanted": input.ResultsWanted,
		"search_term":    input.SearchTerm,
	})

	wanted := input.ResultsWanted
	if wanted <= 0 {
		wanted = 25
	}

	// Obtain token if not configured
	token := s.token
	if token == "" {
		tok, err := s.fetchToken(ctx)
		if err != nil {
			return nil, fmt.Errorf("navjobs: fetch token: %w", err)
		}
		token = tok
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.feedURL, nil)
	if err != nil {
		return nil, fmt.Errorf("navjobs: build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("navjobs: request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("navjobs: status %d", resp.StatusCode)
	}

	body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("navjobs: read: %w", err)
	}

	var parsed apiResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("navjobs: decode: %w", err)
	}

	items := parsed.Items
	if len(items) == 0 {
		return nil, fmt.Errorf("navjobs: no items returned")
	}

	term := strings.ToLower(strings.TrimSpace(input.SearchTerm))

	out := make([]model.JobPost, 0, wanted)
	for _, item := range items {
		if len(out) >= wanted {
			break
		}
		title := strings.TrimSpace(item.Title)
		if title == "" {
			continue
		}

		// Client-side search term filtering
		if term != "" {
			business := ""
			desc := ""
			if item.Entry != nil {
				business = strings.ToLower(item.Entry.BusinessName)
				desc = strings.ToLower(item.Entry.Description)
			}
			hay := strings.ToLower(title) + " " + business + " " + desc
			if !strings.Contains(hay, term) {
				continue
			}
		}

		jobURL := strings.TrimSpace(item.URL)
		if jobURL == "" {
			continue
		}

		job := model.JobPost{
			Site:     string(s.SiteName()),
			Location: model.Location{Country: "Norway"},
		}

		// Generate ID from uuid or item.id
		var uid string
		if item.Entry != nil && item.Entry.UUID != "" {
			uid = item.Entry.UUID
		} else {
			uid = item.ID
		}
		job.ID = "navjobs-" + uid

		job.Title = title

		if item.Entry != nil {
			job.CompanyName = strings.TrimSpace(item.Entry.BusinessName)
			job.Description = strings.TrimSpace(item.Entry.Description)

			// Location
			if city := strings.TrimSpace(item.Entry.Municipal); city != "" {
				job.Location.City = city
			}
			if state := strings.TrimSpace(item.Entry.County); state != "" {
				job.Location.State = state
			}
			// Use applicationUrl or sourceurl, fallback to item.url
			if appURL := strings.TrimSpace(item.Entry.ApplicationURL); appURL != "" {
				job.JobURL = appURL
			} else if srcURL := strings.TrimSpace(item.Entry.SourceURL); srcURL != "" {
				job.JobURL = srcURL
			} else {
				job.JobURL = jobURL
			}

			// Date posted
			if pd := strings.TrimSpace(item.Entry.Published); pd != "" {
				job.DatePosted = parseDate(pd)
			}
		} else {
			job.JobURL = jobURL
		}

		out = append(out, job)
	}

	util.Debug("scraper_done", map[string]any{
		"site": s.SiteName(),
		"jobs": len(out),
	})

	if !util.HasMeaningfulJobs(out) {
		return nil, fmt.Errorf("navjobs: no parseable jobs")
	}
	return out, nil
}

// fetchToken obtains a public token from the NAV Jobs token endpoint.
func (s *Scraper) fetchToken(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.tokenURL, nil)
	if err != nil {
		return "", fmt.Errorf("navjobs: token request: %w", err)
	}
	req.Header.Set("Accept", "text/plain")
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("navjobs: token request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("navjobs: token status %d", resp.StatusCode)
	}

	body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return "", fmt.Errorf("navjobs: token read: %w", err)
	}

	token := strings.TrimSpace(string(body))
	if token == "" {
		return "", fmt.Errorf("navjobs: empty token")
	}
	return token, nil
}

type apiResponse struct {
	Items []feedItem `json:"items"`
}

type feedItem struct {
	ID    string   `json:"id"`
	Title string   `json:"title"`
	URL   string   `json:"url"`
	Entry *feedEntry `json:"_feed_entry"`
}

type feedEntry struct {
	UUID           string `json:"uuid"`
	BusinessName   string `json:"businessName"`
	Description    string `json:"description"`
	Municipal      string `json:"municipal"`
	County         string `json:"county"`
	Published      string `json:"published"`
	ApplicationURL string `json:"applicationUrl"`
	SourceURL      string `json:"sourceurl"`
}

func parseDate(v string) *time.Time {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05",
		"2006-01-02",
	}
	for _, f := range formats {
		t, err := time.Parse(f, v)
		if err == nil {
			return &t
		}
	}
	return nil
}
