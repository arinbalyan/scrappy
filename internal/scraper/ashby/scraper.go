package ashby

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/scraper/ats"
	"github.com/arinbalyan/scrappy/internal/util"
)

const ashbyAPIURL = "https://api.ashbyhq.com/posting-api/job-board"

// Scraper fetches jobs from the Ashby posting API.
type Scraper struct {
	client *http.Client
	apiURL string
}

// New creates a new Ashby scraper.
func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Timeout: 20 * time.Second})
	}
	return &Scraper{client: client, apiURL: ashbyAPIURL}
}

// NewWithAPIURL creates a new Ashby scraper with a custom API URL (used in tests).
func NewWithAPIURL(client *http.Client, apiURL string) *Scraper {
	s := New(client)
	if strings.TrimSpace(apiURL) != "" {
		s.apiURL = apiURL
	}
	return s
}

func (s *Scraper) SiteName() model.Site { return model.SiteAshby }

// --- API response types ---

type ashbyAddress struct {
	PostalAddress *ashbyPostalAddress `json:"postalAddress"`
}

type ashbyPostalAddress struct {
	AddressLocality string `json:"addressLocality"`
	AddressRegion   string `json:"addressRegion"`
	AddressCountry  string `json:"addressCountry"`
}

type ashbyCompensationTier struct {
	Title       string   `json:"title"`
	TierFloor   *float64 `json:"tierFloor"`
	TierCeiling *float64 `json:"tierCeiling"`
	Currency    string   `json:"currency"`
	TierType    string   `json:"tierType"`
	Interval    string   `json:"interval"`
}

type ashbyCompensationComponent struct {
	CompensationType string                    `json:"compensationType"`
	Tiers            []ashbyCompensationTier   `json:"tiers"`
	Label            string                    `json:"label"`
}

type ashbyCompensation struct {
	CompensationComponents []ashbyCompensationComponent `json:"compensationComponents"`
	SummaryComponents      []ashbyCompensationComponent `json:"summaryComponents"`
}

type ashbyJob struct {
	ID               string             `json:"id"`
	Title            string             `json:"title"`
	DepartmentName   string             `json:"departmentName"`
	TeamName         string             `json:"teamName"`
	EmploymentType   string             `json:"employmentType"`
	Location         string             `json:"location"`
	Address          *ashbyAddress      `json:"address"`
	IsRemote         *bool              `json:"isRemote"`
	PublishedDate    string             `json:"publishedDate"`
	DescriptionHTML  string             `json:"descriptionHtml"`
	DescriptionPlain string             `json:"descriptionPlain"`
	JobURL           string             `json:"jobUrl"`
	ApplyURL         string             `json:"applyUrl"`
	Compensation     *ashbyCompensation `json:"compensation"`
	IsListed         *bool              `json:"isListed"`
}

type ashbyResponse struct {
	Jobs       []ashbyJob `json:"jobs"`
	APIVersion string     `json:"apiVersion"`
}

// Scrape fetches jobs from Ashby for the given company seeds.
func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{"site": s.SiteName(), "results_wanted": input.ResultsWanted})

	seeds, src := ats.ResolveSeeds(input.SearchTerm, "SCRAPPY_ASHBY_SEEDS")
	if len(seeds) == 0 {
		return nil, fmt.Errorf("ashby no seeds: set SCRAPPY_ASHBY_SEEDS or pass a company slug in --search")
	}
	util.Debug("ashby_seeds", map[string]any{"seeds": seeds, "src": src})

	wanted := input.ResultsWanted
	if wanted <= 0 {
		wanted = 25
	}

	out := make([]model.JobPost, 0, wanted)
	seen := make(map[string]bool)

	for _, slug := range seeds {
		if len(out) >= wanted {
			break
		}

		u := s.apiURL + "/" + url.PathEscape(slug)
		var resp ashbyResponse
		if err := ats.FetchJSON(ctx, s.client, u, &resp); err != nil {
			util.Warn("ashby_fetch_fail", map[string]any{"slug": slug, "err": err.Error()})
			continue
		}

		for _, job := range resp.Jobs {
			if len(out) >= wanted {
				break
			}
			if job.IsListed != nil && !*job.IsListed {
				continue
			}

			title := strings.TrimSpace(job.Title)
			if title == "" || job.ID == "" {
				continue
			}

			id := ats.BuildID("ashby", slug, job.ID)
			if seen[id] {
				continue
			}
			seen[id] = true

			// Description
			desc := strings.TrimSpace(job.DescriptionPlain)
			if desc == "" && job.DescriptionHTML != "" {
				desc = util.StripHTML(job.DescriptionHTML)
			}

			// Location
			l := model.Location{}
			if job.Address != nil && job.Address.PostalAddress != nil {
				addr := job.Address.PostalAddress
				l.City = strings.TrimSpace(addr.AddressLocality)
				l.State = strings.TrimSpace(addr.AddressRegion)
				l.Country = strings.TrimSpace(addr.AddressCountry)
			}
			if l.City == "" && job.Location != "" {
				l.City = strings.TrimSpace(job.Location)
			}

			isRemote := false
			if job.IsRemote != nil {
				isRemote = *job.IsRemote
			}

			// Job URL
			jobURL := strings.TrimSpace(job.JobURL)
			if jobURL == "" {
				jobURL = fmt.Sprintf("https://jobs.ashbyhq.com/%s/%s", url.PathEscape(slug), job.ID)
			}

			jp := model.JobPost{
				ID:          id,
				Title:       title,
				CompanyName: slug,
				JobURL:      jobURL,
				Location:    l,
				IsRemote:    isRemote,
				Description: desc,
				Site:        string(s.SiteName()),
				Department:  strings.TrimSpace(job.DepartmentName),
				JobType:     normalizeAshbyEmploymentType(job.EmploymentType),
			}

			// Compensation
			if job.Compensation != nil {
				jp.Compensation = extractCompensation(job.Compensation)
			}

			if dp := strings.TrimSpace(job.PublishedDate); dp != "" {
				jp.DatePosted = util.ParseDatePosted(dp)
			}

			out = append(out, jp)
		}
	}

	if !util.HasMeaningfulJobs(out) {
		return nil, fmt.Errorf("ashby no parseable jobs")
	}
	return out, nil
}

func normalizeAshbyEmploymentType(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "fulltime", "full-time":
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

func extractCompensation(comp *ashbyCompensation) *model.Compensation {
	components := comp.CompensationComponents
	if len(components) == 0 {
		components = comp.SummaryComponents
	}
	if len(components) == 0 {
		return nil
	}

	// Find the base salary component
	var target *ashbyCompensationComponent
	for i := range components {
		ct := strings.ToLower(components[i].CompensationType)
		lb := strings.ToLower(components[i].Label)
		if strings.Contains(ct, "salary") || strings.Contains(lb, "salary") || ct == "base" {
			target = &components[i]
			break
		}
	}
	if target == nil {
		target = &components[0]
	}

	if len(target.Tiers) == 0 {
		return nil
	}

	tier := target.Tiers[0]
	if tier.TierFloor == nil && tier.TierCeiling == nil {
		return nil
	}

	currency := strings.TrimSpace(tier.Currency)
	if currency == "" {
		currency = "USD"
	}

	interval := model.IntervalYearly
	switch strings.ToLower(strings.TrimSpace(tier.Interval)) {
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

	return &model.Compensation{
		Interval:  interval,
		MinAmount: tier.TierFloor,
		MaxAmount: tier.TierCeiling,
		Currency:  currency,
	}
}
