package fountain

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

const fountainAPIURL = "https://api.fountain.com/v2/openings"

// Scraper fetches jobs from Fountain ATS.
// Requires FOUNTAIN_API_KEY env var for authentication.
type Scraper struct {
	client *http.Client
	apiURL string
}

// New creates a new Fountain scraper.
func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Timeout: 20 * time.Second})
	}
	return &Scraper{client: client, apiURL: fountainAPIURL}
}

// NewWithAPIURL creates a new Fountain scraper with a custom API URL (used in tests).
func NewWithAPIURL(client *http.Client, apiURL string) *Scraper {
	s := New(client)
	if strings.TrimSpace(apiURL) != "" {
		s.apiURL = apiURL
	}
	return s
}

func (s *Scraper) SiteName() model.Site { return model.SiteFountain }

// --- API response types ---

type fountainLocation struct {
	City     string `json:"city"`
	State    string `json:"state"`
	Country  string `json:"country"`
	Address  string `json:"address"`
	ZipCode  string `json:"zip_code"`
}

type fountainCompensation struct {
	MinAmount *float64 `json:"min_amount"`
	MaxAmount *float64 `json:"max_amount"`
	Currency  string   `json:"currency"`
	Interval  string   `json:"interval"`
}

type fountainOpening struct {
	ID             string                 `json:"id"`
	Title          string                 `json:"title"`
	Description    string                 `json:"description"`
	Location       *fountainLocation      `json:"location"`
	LocationString string                 `json:"location_string"`
	Department     string                 `json:"department"`
	Team           string                 `json:"team"`
	Type           string                 `json:"type"`
	EmploymentType string                 `json:"employment_type"`
	CreatedAt      string                 `json:"created_at"`
	UpdatedAt      string                 `json:"updated_at"`
	URL            string                 `json:"url"`
	ApplyURL       string                 `json:"apply_url"`
	IsRemote       *bool                  `json:"is_remote"`
	Compensation   *fountainCompensation  `json:"compensation"`
	Status         string                 `json:"status"`
	Category       string                 `json:"category"`
}

type fountainResponse struct {
	Openings []fountainOpening `json:"openings"`
}

// Scrape fetches jobs from Fountain.
func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{"site": s.SiteName(), "results_wanted": input.ResultsWanted})

	seeds, src := ats.ResolveSeeds(input.SearchTerm, "SCRAPPY_FOUNTAIN_SEEDS")
	if len(seeds) == 0 {
		return nil, fmt.Errorf("fountain no seeds: set SCRAPPY_FOUNTAIN_SEEDS or pass a company slug in --search")
	}
	util.Debug("fountain_seeds", map[string]any{"seeds": seeds, "src": src})

	// Fountain requires a Bearer token via env var
	apiKey := ""
	for _, slug := range seeds {
		// The slug IS the API key for Fountain
		apiKey = slug
		break
	}
	if apiKey == "" {
		return nil, fmt.Errorf("fountain no API key available")
	}

	wanted := input.ResultsWanted
	if wanted <= 0 {
		wanted = 25
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("fountain request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fountain fetch: %w", err)
	}

	body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	resp.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("fountain read: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("fountain status %d", resp.StatusCode)
	}

	var apiResp fountainResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("fountain decode: %w", err)
	}

	out := make([]model.JobPost, 0, wanted)
	seen := make(map[string]bool)

	for _, opening := range apiResp.Openings {
		if len(out) >= wanted {
			break
		}

		title := strings.TrimSpace(opening.Title)
		if title == "" || opening.ID == "" {
			continue
		}

		id := ats.BuildID("fountain", "fountain", opening.ID)
		if seen[id] {
			continue
		}
		seen[id] = true

		// Location
		l := model.Location{}
		isRemote := false
		if opening.Location != nil {
			l.City = strings.TrimSpace(opening.Location.City)
			l.State = strings.TrimSpace(opening.Location.State)
			l.Country = strings.TrimSpace(opening.Location.Country)
		}
		if l.City == "" && opening.LocationString != "" {
			l.City = strings.TrimSpace(opening.LocationString)
		}
		if opening.IsRemote != nil {
			isRemote = *opening.IsRemote
		}

		// Job URL
		jobURL := strings.TrimSpace(opening.URL)
		if jobURL == "" {
			jobURL = strings.TrimSpace(opening.ApplyURL)
		}

		jp := model.JobPost{
			ID:          id,
			Title:       title,
			JobURL:      jobURL,
			Location:    l,
			IsRemote:    isRemote,
			Description: util.StripHTML(strings.TrimSpace(opening.Description)),
			Site:        string(s.SiteName()),
			Department:  strings.TrimSpace(opening.Department),
			JobType:     normalizeEmploymentType(opening.EmploymentType, opening.Type),
		}

		// Compensation
		if opening.Compensation != nil {
			jp.Compensation = extractCompensation(opening.Compensation)
		}

		if dp := strings.TrimSpace(opening.CreatedAt); dp != "" {
			jp.DatePosted = util.ParseDatePosted(dp)
		}

		out = append(out, jp)
	}

	if !util.HasMeaningfulJobs(out) {
		return nil, fmt.Errorf("fountain no parseable jobs")
	}
	return out, nil
}

func normalizeEmploymentType(employmentType, openingType string) string {
	v := strings.ToLower(strings.TrimSpace(employmentType))
	if v != "" {
		switch {
		case strings.Contains(v, "full"):
			return "fulltime"
		case strings.Contains(v, "part"):
			return "parttime"
		case strings.Contains(v, "contract"), strings.Contains(v, "temp"):
			return "contract"
		case strings.Contains(v, "intern"):
			return "internship"
		}
	}
	v = strings.ToLower(strings.TrimSpace(openingType))
	switch v {
	case "fulltime", "full-time", "permanent":
		return "fulltime"
	case "parttime", "part-time":
		return "parttime"
	case "contract", "contractor":
		return "contract"
	case "internship", "intern":
		return "internship"
	}
	return v
}

func extractCompensation(comp *fountainCompensation) *model.Compensation {
	if comp.MinAmount == nil && comp.MaxAmount == nil {
		return nil
	}

	interval := model.IntervalHourly
	switch strings.ToLower(strings.TrimSpace(comp.Interval)) {
	case "yearly", "year", "annual", "per year":
		interval = model.IntervalYearly
	case "monthly", "per month", "month":
		interval = model.IntervalMonthly
	case "weekly", "per week", "week":
		interval = model.IntervalWeekly
	case "daily", "per day", "day":
		interval = model.IntervalDaily
	case "hourly", "per hour", "hour":
		interval = model.IntervalHourly
	}

	currency := strings.TrimSpace(comp.Currency)
	if currency == "" {
		currency = "USD"
	}

	return &model.Compensation{
		Interval:  interval,
		MinAmount: comp.MinAmount,
		MaxAmount: comp.MaxAmount,
		Currency:  currency,
	}
}
