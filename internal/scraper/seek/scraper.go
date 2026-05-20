package seek

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/util"
)

const defaultAPI = "https://www.seek.com.au/api/chalice-search/v4/search"

type Scraper struct {
	client *http.Client
	apiURL string
}

func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Retries: 3, CookieResetEveryN: 160, Timeout: 20 * time.Second})
	}
	return &Scraper{client: client, apiURL: defaultAPI}
}

func NewWithAPIURL(client *http.Client, endpoint string) *Scraper {
	s := New(client)
	if strings.TrimSpace(endpoint) != "" {
		s.apiURL = endpoint
	}
	return s
}

func (s *Scraper) SiteName() model.Site { return model.SiteSeek }

func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	u, _ := url.Parse(s.apiURL)
	q := u.Query()
	if input.SearchTerm != "" {
		q.Set("keywords", input.SearchTerm)
	}
	if input.Location != "" {
		q.Set("where", input.Location)
	}
	q.Set("page", "1")
	u.RawQuery = q.Encode()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	req.Header.Set("Accept", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("seek request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("seek status %d", resp.StatusCode)
	}

	var parsed struct {
		Data []struct {
			ID         string `json:"id"`
			Title      string `json:"title"`
			Teaser     string `json:"teaser"`
			Advertiser struct {
				Description string `json:"description"`
			} `json:"advertiser"`
			Location    string `json:"location"`
			ListingDate string `json:"listingDate"`
			JobURL      string `json:"jobUrl"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("seek decode: %w", err)
	}
	limit := input.ResultsWanted
	if limit <= 0 || limit > len(parsed.Data) {
		limit = len(parsed.Data)
	}
	jobs := make([]model.JobPost, 0, limit)
	for i := 0; i < limit; i++ {
		r := parsed.Data[i]
		var posted *time.Time
		if t, err := time.Parse(time.RFC3339, r.ListingDate); err == nil {
			posted = &t
		}
		jobs = append(jobs, model.JobPost{ID: "seek-" + strings.TrimSpace(r.ID), Title: strings.TrimSpace(r.Title), CompanyName: strings.TrimSpace(r.Advertiser.Description), JobURL: strings.TrimSpace(r.JobURL), Description: strings.TrimSpace(r.Teaser), Location: model.Location{City: strings.TrimSpace(r.Location)}, DatePosted: posted, IsRemote: strings.Contains(strings.ToLower(r.Location), "remote")})
	}
	return jobs, nil
}
