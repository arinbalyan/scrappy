package pinpoint

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

func (s *Scraper) SiteName() model.Site { return model.SitePinpoint }

type pinpointListing struct {
	ID         interface{} `json:"id"`
	Attributes *pinpointAttrs `json:"attributes,omitempty"`
	URL        string         `json:"url,omitempty"`
}

type pinpointAttrs struct {
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Location    string `json:"location,omitempty"`
	Remote      bool   `json:"remote,omitempty"`
	URL         string `json:"url,omitempty"`
	CompanyName string `json:"company_name,omitempty"`
	Department  string `json:"department_name,omitempty"`
	PublishedAt string `json:"published_at,omitempty"`
	CreatedAt   string `json:"created_at,omitempty"`
}

type pinpointResponse struct {
	Data []pinpointListing `json:"data"`
}

func (s *Scraper) buildURL(slug string) string {
	if s.apiURL != "" {
		return s.apiURL
	}
	return fmt.Sprintf("https://%s.pinpointhq.com/postings.json", slug)
}

func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{"site": s.SiteName(), "results_wanted": input.ResultsWanted})

	seeds, src := ats.ResolveSeeds(input.SearchTerm, "SCRAPPY_PINPOINT_SEEDS")
	if len(seeds) == 0 {
		return nil, fmt.Errorf("pinpoint no seeds: set SCRAPPY_PINPOINT_SEEDS or pass a company slug in --search")
	}
	util.Debug("pinpoint_seeds", map[string]any{"seeds": seeds, "src": src})

	wanted := input.ResultsWanted
	if wanted <= 0 {
		wanted = 100
	}

	out := make([]model.JobPost, 0, wanted)
	for _, slug := range seeds {
		if len(out) >= wanted {
			break
		}

		u := s.buildURL(slug)

		body, err := s.fetchRaw(ctx, u)
		if err != nil {
			util.Warn("pinpoint_fetch_fail", map[string]any{"slug": slug, "err": err.Error()})
			continue
		}

		listings, err := s.parseResponse(body)
		if err != nil {
			util.Warn("pinpoint_parse_fail", map[string]any{"slug": slug, "err": err.Error()})
			continue
		}

		for _, listing := range listings {
			if len(out) >= wanted {
				break
			}
			jp := s.toJobPost(listing, slug)
			if jp != nil {
				out = append(out, *jp)
			}
		}
	}

	if !util.HasMeaningfulJobs(out) {
		return nil, fmt.Errorf("pinpoint no parseable jobs")
	}
	return out, nil
}

func (s *Scraper) fetchRaw(ctx context.Context, u string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}

	return util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
}

func (s *Scraper) parseResponse(body []byte) ([]pinpointListing, error) {
	// Pinpoint can return either {"data": [...]} or a bare array
	var wrapped pinpointResponse
	if err := json.Unmarshal(body, &wrapped); err == nil && len(wrapped.Data) > 0 {
		return wrapped.Data, nil
	}

	var arr []pinpointListing
	if err := json.Unmarshal(body, &arr); err == nil {
		return arr, nil
	}

	return nil, fmt.Errorf("unable to parse Pinpoint response")
}

func (s *Scraper) toJobPost(listing pinpointListing, slug string) *model.JobPost {
	attrs := listing.Attributes
	if attrs == nil {
		return nil
	}
	title := strings.TrimSpace(attrs.Title)
	if title == "" {
		return nil
	}

	loc := model.Location{}
	isRemote := attrs.Remote
	locStr := strings.TrimSpace(attrs.Location)
	if locStr != "" {
		loc.City = locStr
		if !isRemote && strings.Contains(strings.ToLower(locStr), "remote") {
			isRemote = true
		}
	}

	jobURL := strings.TrimSpace(attrs.URL)
	if jobURL == "" {
		jobID := ""
		if listing.ID != nil {
			jobID = fmt.Sprintf("%v", listing.ID)
		}
		jobURL = fmt.Sprintf("https://%s.pinpointhq.com/postings/%s", slug, jobID)
	}

	company := strings.TrimSpace(attrs.CompanyName)
	if company == "" {
		company = slug
	}

	dateStr := strings.TrimSpace(attrs.PublishedAt)
	if dateStr == "" {
		dateStr = strings.TrimSpace(attrs.CreatedAt)
	}

	id := fmt.Sprintf("pinpoint-%s-%v", slug, listing.ID)
	desc := util.StripHTML(strings.TrimSpace(attrs.Description))

	jp := &model.JobPost{
		ID:          id,
		Title:       title,
		CompanyName: company,
		JobURL:      jobURL,
		Location:    loc,
		IsRemote:    isRemote,
		Description: desc,
		Site:        string(model.SitePinpoint),
		Department:  strings.TrimSpace(attrs.Department),
	}
	if dateStr != "" {
		jp.DatePosted = util.ParseDatePosted(dateStr)
	}
	return jp
}
