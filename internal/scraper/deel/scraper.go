package deel

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

const deelAPIURL = "https://api.letsdeel.com/rest/v2/ats/job-postings"

// Scraper fetches jobs from Deel ATS.
// Requires DEEL_API_TOKEN env var for authentication.
type Scraper struct {
	client *http.Client
	apiURL string
}

// New creates a new Deel scraper.
func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Timeout: 20 * time.Second})
	}
	return &Scraper{client: client, apiURL: deelAPIURL}
}

// NewWithAPIURL creates a new Deel scraper with a custom API URL (used in tests).
func NewWithAPIURL(client *http.Client, apiURL string) *Scraper {
	s := New(client)
	if strings.TrimSpace(apiURL) != "" {
		s.apiURL = apiURL
	}
	return s
}

func (s *Scraper) SiteName() model.Site { return model.SiteDeel }

// --- API response types ---

type deelLocation struct {
	City    string `json:"city"`
	State   string `json:"state"`
	Country string `json:"country"`
}

type deelSalary struct {
	MinAmount *float64 `json:"min_amount"`
	MaxAmount *float64 `json:"max_amount"`
	Currency  string   `json:"currency"`
	Interval  string   `json:"interval"`
}

type deelJobPosting struct {
	ID             string        `json:"id"`
	Title          string        `json:"title"`
	Description    string        `json:"description"`
	Location       *deelLocation `json:"location"`
	Department     string        `json:"department"`
	EmploymentType string        `json:"employment_type"`
	CreatedAt      string        `json:"created_at"`
	UpdatedAt      string        `json:"updated_at"`
	URL            string        `json:"url"`
	ApplyURL       string        `json:"apply_url"`
	Salary         *deelSalary   `json:"salary"`
	Status         string        `json:"status"`
	Remote         *bool         `json:"remote"`
	CompanyName    string        `json:"company_name"`
	Team           string        `json:"team"`
}

type deelResponse struct {
	Data []deelJobPosting `json:"data"`
}

// Scrape fetches jobs from Deel ATS.
func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{"site": s.SiteName(), "results_wanted": input.ResultsWanted})

	seeds, src := ats.ResolveSeeds(input.SearchTerm, "SCRAPPY_DEEL_SEEDS")
	if len(seeds) == 0 {
		return nil, fmt.Errorf("deel no seeds: set SCRAPPY_DEEL_SEEDS or pass a company slug in --search")
	}
	util.Debug("deel_seeds", map[string]any{"seeds": seeds, "src": src})

	// Deel requires a Bearer token via env var
	apiToken := ""
	for _, slug := range seeds {
		// The slug IS the API token for Deel
		apiToken = slug
		break
	}
	if apiToken == "" {
		return nil, fmt.Errorf("deel no API token available")
	}

	wanted := input.ResultsWanted
	if wanted <= 0 {
		wanted = 25
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("deel request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiToken)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("deel fetch: %w", err)
	}

	body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	resp.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("deel read: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("deel status %d", resp.StatusCode)
	}

	var apiResp deelResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("deel decode: %w", err)
	}

	out := make([]model.JobPost, 0, wanted)
	seen := make(map[string]bool)

	for _, posting := range apiResp.Data {
		if len(out) >= wanted {
			break
		}

		title := strings.TrimSpace(posting.Title)
		if title == "" || posting.ID == "" {
			continue
		}

		id := ats.BuildID("deel", "deel", posting.ID)
		if seen[id] {
			continue
		}
		seen[id] = true

		// Location
		l := model.Location{}
		if posting.Location != nil {
			l.City = strings.TrimSpace(posting.Location.City)
			l.State = strings.TrimSpace(posting.Location.State)
			l.Country = strings.TrimSpace(posting.Location.Country)
		}

		isRemote := false
		if posting.Remote != nil {
			isRemote = *posting.Remote
		}

		// Job URL
		jobURL := strings.TrimSpace(posting.URL)
		if jobURL == "" {
			jobURL = strings.TrimSpace(posting.ApplyURL)
		}

		jp := model.JobPost{
			ID:          id,
			Title:       title,
			CompanyName: strings.TrimSpace(posting.CompanyName),
			JobURL:      jobURL,
			Location:    l,
			IsRemote:    isRemote,
			Description: util.StripHTML(strings.TrimSpace(posting.Description)),
			Site:        string(s.SiteName()),
			Department:  strings.TrimSpace(posting.Department),
			JobType:     normalizeEmploymentType(posting.EmploymentType),
		}

		// Compensation
		if posting.Salary != nil {
			jp.Compensation = extractCompensation(posting.Salary)
		}

		if dp := strings.TrimSpace(posting.CreatedAt); dp != "" {
			jp.DatePosted = util.ParseDatePosted(dp)
		}

		out = append(out, jp)
	}

	if !util.HasMeaningfulJobs(out) {
		return nil, fmt.Errorf("deel no parseable jobs")
	}
	return out, nil
}

func normalizeEmploymentType(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "fulltime", "full-time", "permanent":
		return "fulltime"
	case "parttime", "part-time":
		return "parttime"
	case "contract", "contractor", "temporary":
		return "contract"
	case "internship", "intern":
		return "internship"
	}
	return v
}

func extractCompensation(salary *deelSalary) *model.Compensation {
	if salary.MinAmount == nil && salary.MaxAmount == nil {
		return nil
	}

	interval := model.IntervalYearly
	switch strings.ToLower(strings.TrimSpace(salary.Interval)) {
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

	currency := strings.TrimSpace(salary.Currency)
	if currency == "" {
		currency = "USD"
	}

	return &model.Compensation{
		Interval:  interval,
		MinAmount: salary.MinAmount,
		MaxAmount: salary.MaxAmount,
		Currency:  currency,
	}
}
