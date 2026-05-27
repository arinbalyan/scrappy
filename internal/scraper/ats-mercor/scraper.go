package mercor

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/scraper/ats"
	"github.com/arinbalyan/scrappy/internal/util"
)

const (
	mercorAPIBaseURL  = "https://aws.api.mercor.com"
	mercorExplorePath = "/work/listings-explore-page"
	mercorPublicOrigin = "https://work.mercor.com"
)

type Scraper struct {
	client *http.Client
	apiURL string
}

func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Timeout: 20 * time.Second})
	}
	return &Scraper{client: client}
}

func NewWithAPIURL(client *http.Client, apiURL string) *Scraper {
	s := New(client)
	if strings.TrimSpace(apiURL) != "" {
		s.apiURL = apiURL
	}
	return s
}

func (s *Scraper) SiteName() model.Site { return model.SiteMercor }

type mercorListing struct {
	ListingID         string  `json:"listingId"`
	Title             string  `json:"title"`
	CompanyName       string  `json:"companyName,omitempty"`
	Location          string  `json:"location,omitempty"`
	PostedAt          string  `json:"postedAt,omitempty"`
	RateMin           float64 `json:"rateMin,omitempty"`
	RateMax           float64 `json:"rateMax,omitempty"`
	PayRateFrequency  string  `json:"payRateFrequency,omitempty"`
	ListingDomain     string  `json:"listingDomain,omitempty"`
	Commitment        string  `json:"commitment,omitempty"`
}

type mercorListingsResponse struct {
	Listings []mercorListing `json:"listings"`
}

func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{"site": s.SiteName(), "results_wanted": input.ResultsWanted})

	seeds, src := ats.ResolveSeeds(input.SearchTerm, "SCRAPPY_MERCOR_SEEDS")
	if len(seeds) > 0 {
		util.Debug("mercor_seeds", map[string]any{"seeds": seeds, "src": src})
	}

	wanted := input.ResultsWanted
	if wanted <= 0 {
		wanted = 100
	}

	url := s.buildURL()
	resp := new(mercorListingsResponse)
	if err := ats.FetchJSON(ctx, s.client, url, resp); err != nil {
		return nil, fmt.Errorf("mercor fetch: %w", err)
	}

	if resp.Listings == nil {
		return nil, fmt.Errorf("mercor no listings in response")
	}

	slugFilter := ""
	if len(seeds) > 0 {
		slugFilter = strings.ToLower(strings.TrimSpace(seeds[0]))
	}

	var filtered []mercorListing
	for _, l := range resp.Listings {
		if slugFilter != "" {
			cn := strings.ToLower(l.CompanyName)
			if !strings.Contains(cn, slugFilter) {
				continue
			}
		}
		filtered = append(filtered, l)
	}

	if len(filtered) > wanted {
		filtered = filtered[:wanted]
	}

	out := make([]model.JobPost, 0, len(filtered))
	for _, l := range filtered {
		jp := s.toJobPost(l)
		out = append(out, jp)
	}

	if !util.HasMeaningfulJobs(out) {
		return nil, fmt.Errorf("mercor no parseable jobs")
	}
	return out, nil
}

func (s *Scraper) buildURL() string {
	if s.apiURL != "" {
		return s.apiURL
	}
	return mercorAPIBaseURL + mercorExplorePath
}

func (s *Scraper) toJobPost(l mercorListing) model.JobPost {
	loc := model.Location{}
	isRemote := false
	if l.Location != "" {
		loc.City = l.Location
		if strings.Contains(strings.ToLower(l.Location), "remote") {
			isRemote = true
		}
	}

	jobURL := fmt.Sprintf("%s/jobs/%s/%s", mercorPublicOrigin, l.ListingID, slugify(l.Title))

	jp := model.JobPost{
		ID:          "mercor-" + l.ListingID,
		Title:       l.Title,
		CompanyName: nonEmpty(l.CompanyName, "Mercor"),
		JobURL:      jobURL,
		Location:    loc,
		IsRemote:    isRemote,
		Site:        string(model.SiteMercor),
	}

	if l.PostedAt != "" {
		jp.DatePosted = util.ParseDatePosted(l.PostedAt)
	}

	if l.RateMin != 0 || l.RateMax != 0 {
		interval := model.IntervalYearly
		switch strings.ToLower(l.PayRateFrequency) {
		case "yearly", "annual":
			interval = model.IntervalYearly
		case "monthly":
			interval = model.IntervalMonthly
		case "weekly":
			interval = model.IntervalWeekly
		case "hourly":
			interval = model.IntervalHourly
		}
		minAmt := l.RateMin
		maxAmt := l.RateMax
		jp.Compensation = &model.Compensation{
			Interval:  interval,
			MinAmount: &minAmt,
			MaxAmount: &maxAmt,
			Currency:  "USD",
		}
	}

	return jp
}

func slugify(text string) string {
	s := strings.ToLower(text)
	s = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == ' ' || r == '_' {
			return r
		}
		return -1
	}, s)
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "_", "-")
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	s = strings.Trim(s, "-")
	return s
}

func nonEmpty(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}
