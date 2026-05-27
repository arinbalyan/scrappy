package functionalworks

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/util"
)

const apiURL = "https://functional.works-hub.com/api/graphql"

// GraphQL query to fetch functional programming jobs.
const graphQLQuery = `{ jobs(page_size:20, vertical:"functional", published:true) { title company { name } location { city country } remote remuneration { timePeriod competitive currency min max } slug firstPublished descriptionHtml tags { label } } }`

// Scraper fetches jobs from the Functional Works GraphQL API.
type Scraper struct {
	client *http.Client
	apiURL string
}

// New creates a new Functional Works scraper. If client is nil a default one is used.
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
func (s *Scraper) SiteName() model.Site { return model.SiteFunctionalWorks }

// --- GraphQL response types ---

type graphQLResponse struct {
	Data *graphQLData `json:"data,omitempty"`
}

type graphQLData struct {
	Jobs []graphQLJob `json:"jobs"`
}

type graphQLJob struct {
	Title           string           `json:"title"`
	Company         *graphQLCompany   `json:"company,omitempty"`
	Location        *graphQLLocation  `json:"location,omitempty"`
	Remote          bool             `json:"remote"`
	Remuneration    *graphQLRemun     `json:"remuneration,omitempty"`
	Slug            string           `json:"slug"`
	FirstPublished  string           `json:"firstPublished"`
	DescriptionHTML string           `json:"descriptionHtml"`
	Tags            []graphQLTag     `json:"tags"`
}

type graphQLCompany struct {
	Name string `json:"name"`
}

type graphQLLocation struct {
	City    string `json:"city"`
	Country string `json:"country"`
}

type graphQLRemun struct {
	TimePeriod  string   `json:"timePeriod"`
	Competitive bool     `json:"competitive"`
	Currency    string   `json:"currency"`
	Min         *float64 `json:"min"`
	Max         *float64 `json:"max"`
}

type graphQLTag struct {
	Label string `json:"label"`
}

// graphQLBody is the request payload sent to the GraphQL endpoint.
type graphQLBody struct {
	Query string `json:"query"`
}

// Scrape fetches jobs from the Functional Works GraphQL API.
func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{
		"site":           s.SiteName(),
		"results_wanted": input.ResultsWanted,
		"search_term":    input.SearchTerm,
	})

	bodyPayload := graphQLBody{Query: graphQLQuery}
	bodyBytes, err := json.Marshal(bodyPayload)
	if err != nil {
		return nil, fmt.Errorf("functionalworks: marshal body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.apiURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("functionalworks: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("functionalworks: request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("functionalworks: status %d", resp.StatusCode)
	}

	raw, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("functionalworks: read: %w", err)
	}

	var gqlResp graphQLResponse
	if err := json.Unmarshal(raw, &gqlResp); err != nil {
		return nil, fmt.Errorf("functionalworks: decode: %w", err)
	}

	if gqlResp.Data == nil || len(gqlResp.Data.Jobs) == 0 {
		return nil, fmt.Errorf("functionalworks: no jobs returned")
	}

	wanted := input.ResultsWanted
	if wanted <= 0 {
		wanted = len(gqlResp.Data.Jobs)
	}
	if wanted > len(gqlResp.Data.Jobs) {
		wanted = len(gqlResp.Data.Jobs)
	}

	term := strings.ToLower(strings.TrimSpace(input.SearchTerm))

	out := make([]model.JobPost, 0, wanted)
	for _, j := range gqlResp.Data.Jobs {
		if len(out) >= wanted {
			break
		}

		title := strings.TrimSpace(j.Title)
		slug := strings.TrimSpace(j.Slug)
		if title == "" || slug == "" {
			continue
		}

		// Client-side search filtering
		if term != "" {
			companyName := ""
			if j.Company != nil {
				companyName = strings.ToLower(j.Company.Name)
			}
			tagText := ""
			for _, t := range j.Tags {
				tagText += " " + strings.ToLower(t.Label)
			}
			hay := strings.ToLower(title) + " " + companyName + " " + tagText
			if !strings.Contains(hay, term) {
				continue
			}
		}

		job := model.JobPost{
			ID:     "functionalworks-" + slug,
			Title:  title,
			JobURL: "https://functional.works-hub.com/jobs/" + slug,
			Site:   string(s.SiteName()),
		}

		// Company
		if j.Company != nil {
			job.CompanyName = strings.TrimSpace(j.Company.Name)
		}

		// Location
		if j.Location != nil {
			city := strings.TrimSpace(j.Location.City)
			country := strings.TrimSpace(j.Location.Country)
			if city != "" || country != "" {
				job.Location = model.Location{City: city, Country: country}
			}
		}

		// Remote
		job.IsRemote = j.Remote

		// Compensation
		if j.Remuneration != nil && !j.Remuneration.Competitive && (j.Remuneration.Min != nil || j.Remuneration.Max != nil) {
			interval := model.IntervalYearly
			tp := strings.ToLower(strings.TrimSpace(j.Remuneration.TimePeriod))
			if strings.Contains(tp, "month") {
				interval = model.IntervalMonthly
			} else if strings.Contains(tp, "day") {
				interval = model.IntervalDaily
			} else if strings.Contains(tp, "hour") {
				interval = model.IntervalHourly
			}
			currency := strings.TrimSpace(j.Remuneration.Currency)
			if currency == "" {
				currency = "GBP"
			}
			job.Compensation = &model.Compensation{
				Interval:  interval,
				MinAmount: j.Remuneration.Min,
				MaxAmount: j.Remuneration.Max,
				Currency:  currency,
			}
		}

		// Build description from tags + descriptionHtml
		descParts := make([]string, 0, 2)
		if len(j.Tags) > 0 {
			labels := make([]string, 0, len(j.Tags))
			for _, t := range j.Tags {
				if strings.TrimSpace(t.Label) != "" {
					labels = append(labels, strings.TrimSpace(t.Label))
				}
			}
			if len(labels) > 0 {
				descParts = append(descParts, "Tags: "+strings.Join(labels, ", "))
			}
		}
		if strings.TrimSpace(j.DescriptionHTML) != "" {
			plain := util.StripHTML(j.DescriptionHTML)
			if strings.TrimSpace(plain) != "" {
				descParts = append(descParts, strings.TrimSpace(plain))
			}
		}
		if len(descParts) > 0 {
			job.Description = strings.Join(descParts, "\n")
		}

		// DatePosted
		if strings.TrimSpace(j.FirstPublished) != "" {
			job.DatePosted = parseISO(j.FirstPublished)
		}

		out = append(out, job)
	}

	util.Debug("scraper_done", map[string]any{
		"site": s.SiteName(),
		"jobs": len(out),
	})

	if !util.HasMeaningfulJobs(out) {
		return nil, fmt.Errorf("functionalworks: no parseable jobs")
	}
	return out, nil
}

// parseISO parses an ISO 8601 / RFC3339 date string.
func parseISO(v string) *time.Time {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	if t, err := time.Parse(time.RFC3339, v); err == nil {
		return &t
	}
	if t, err := time.Parse("2006-01-02", v); err == nil {
		return &t
	}
	return nil
}
