package nofluffjobs

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

const apiURL = "https://nofluffjobs.com/api/posting"

// posting maps the JSON shape returned by the NoFluffJobs public API.
type posting struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Title      string   `json:"title"`
	Technology string   `json:"technology"`
	Category   string   `json:"category"`
	Seniority  []string `json:"seniority"`
	Location   struct {
		Places []struct {
			Country struct {
				Code string `json:"code"`
				Name string `json:"name"`
			} `json:"country"`
			City string `json:"city"`
		} `json:"places"`
		FullyRemote bool `json:"fullyRemote"`
	} `json:"location"`
	Salary struct {
		From     float64 `json:"from"`
		To       float64 `json:"to"`
		Currency string  `json:"currency"`
		Type     string  `json:"type"`
	} `json:"salary"`
	Posted  int64    `json:"posted"`
	URL     string   `json:"url"`
	Regions []string `json:"regions"`
}

// Scraper fetches jobs from the NoFluffJobs public API.
type Scraper struct {
	client *http.Client
	apiURL string
}

// New creates a new NoFluffJobs scraper. If client is nil a default one is used.
func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Timeout: 20 * time.Second})
	}
	return &Scraper{client: client, apiURL: apiURL}
}

// NewWithAPIURL creates a new scraper with a custom endpoint (used in tests).
func NewWithAPIURL(client *http.Client, endpoint string) *Scraper {
	s := New(client)
	if strings.TrimSpace(endpoint) != "" {
		s.apiURL = endpoint
	}
	return s
}

// SiteName returns the site identifier.
func (s *Scraper) SiteName() model.Site { return model.SiteNoFluffJobs }

// Scrape fetches jobs from the NoFluffJobs public API.
func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{
		"site":           s.SiteName(),
		"results_wanted": input.ResultsWanted,
		"search_term":    input.SearchTerm,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("nofluffjobs: build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("nofluffjobs: request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("nofluffjobs: status %d", resp.StatusCode)
	}

	const maxNoFluffBodyBytes = 4 * 1024 * 1024
	body, err := util.ReadBodyLimited(resp.Body, maxNoFluffBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("nofluffjobs: read: %w", err)
	}

	var postings []posting
	if err := json.Unmarshal(body, &postings); err != nil {
		return nil, fmt.Errorf("nofluffjobs: decode: %w", err)
	}

	term := strings.ToLower(strings.TrimSpace(input.SearchTerm))
	wanted := input.ResultsWanted
	if wanted <= 0 {
		wanted = len(postings)
	}
	if wanted > len(postings) {
		wanted = len(postings)
	}

	out := make([]model.JobPost, 0, wanted)
	for _, p := range postings {
		if len(out) >= wanted {
			break
		}
		title := strings.TrimSpace(p.Title)
		if title == "" {
			continue
		}

		// Client-side search term filtering on title, technology, category
		if term != "" {
			hay := strings.ToLower(title + " " + p.Technology + " " + p.Category)
			if !strings.Contains(hay, term) {
				continue
			}
		}

		job := model.JobPost{
			ID:       "nofluffjobs-" + p.ID,
			Title:    title,
			Site:     string(s.SiteName()),
			IsRemote: p.Location.FullyRemote,
		}

		// Company name from "name" field
		if strings.TrimSpace(p.Name) != "" {
			job.CompanyName = strings.TrimSpace(p.Name)
		}

		// Build job URL
		jobURL := strings.TrimSpace(p.URL)
		if jobURL != "" {
			job.JobURL = "https://nofluffjobs.com/job/" + jobURL
		} else {
			job.JobURL = "https://nofluffjobs.com/job/" + p.ID
		}

		// Location from first places[] entry
		if len(p.Location.Places) > 0 {
			pl := p.Location.Places[0]
			job.Location = model.Location{
				City:    strings.TrimSpace(pl.City),
				Country: strings.TrimSpace(pl.Country.Name),
			}
		}

		// Compensation
		if p.Salary.From > 0 || p.Salary.To > 0 {
			minAmt := p.Salary.From
			maxAmt := p.Salary.To
			curr := p.Salary.Currency
			if curr == "" {
				curr = "PLN"
			}
			job.Compensation = &model.Compensation{
				Interval:  model.IntervalYearly,
				MinAmount: &minAmt,
				MaxAmount: &maxAmt,
				Currency:  curr,
			}
		}

		// DatePosted from Unix epoch milliseconds
		if p.Posted > 0 {
			t := time.UnixMilli(p.Posted)
			job.DatePosted = &t
		}

		// Build description from available fields
		descParts := make([]string, 0, 4)
		if p.Category != "" {
			descParts = append(descParts, "Category: "+p.Category)
		}
		if p.Technology != "" {
			descParts = append(descParts, "Technology: "+p.Technology)
		}
		if len(p.Seniority) > 0 {
			descParts = append(descParts, "Seniority: "+strings.Join(p.Seniority, ", "))
		}
		if len(p.Regions) > 0 {
			descParts = append(descParts, "Regions: "+strings.Join(p.Regions, ", "))
		}
		if len(descParts) > 0 {
			job.Description = strings.Join(descParts, "\n")
		}

		out = append(out, job)
	}

	util.Debug("scraper_done", map[string]any{
		"site": s.SiteName(),
		"jobs": len(out),
	})

	if !util.HasMeaningfulJobs(out) {
		return nil, fmt.Errorf("nofluffjobs: no parseable jobs")
	}
	return out, nil
}
