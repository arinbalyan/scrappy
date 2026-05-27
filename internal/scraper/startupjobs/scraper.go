package startupjobs

import (
	"bytes"
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

const (
	defaultAPIURL  = "https://startup.jobs/api/jobs"
	defaultFeedURL = "https://startup.jobs/feed.json"
)

type Scraper struct {
	client  *http.Client
	apiURL  string
	feedURL string
}

func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Retries: 3, Timeout: 18 * time.Second})
	}
	return &Scraper{client: client, apiURL: defaultAPIURL, feedURL: defaultFeedURL}
}

func NewWithURLs(client *http.Client, apiURL, feedURL string) *Scraper {
	s := New(client)
	if strings.TrimSpace(apiURL) != "" {
		s.apiURL = apiURL
	}
	if strings.TrimSpace(feedURL) != "" {
		s.feedURL = feedURL
	}
	return s
}

func (s *Scraper) SiteName() model.Site { return model.SiteStartupJobs }

func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	wanted := input.ResultsWanted
	if wanted <= 0 {
		wanted = 25
	}
	raw, err := s.fetch(ctx, s.apiURL)
	if err != nil || len(raw) == 0 {
		raw, err = s.fetch(ctx, s.feedURL)
		if err != nil {
			return nil, err
		}
	}
	if len(raw) == 0 {
		return nil, fmt.Errorf("startupjobs no parseable jobs")
	}
	term := strings.ToLower(strings.TrimSpace(input.SearchTerm))
	out := make([]model.JobPost, 0, wanted)
	for i, r := range raw {
		if len(out) >= wanted {
			break
		}
		title := strings.TrimSpace(r.Title)
		if title == "" {
			continue
		}
		if term != "" && !strings.Contains(strings.ToLower(title), term) {
			continue
		}
		company := strings.TrimSpace(firstNonEmpty(r.CompanyName, r.Company.Name))
		if company == "" {
			company = "Unknown Employer"
		}
		jobURL := strings.TrimSpace(r.URL)
		if jobURL == "" {
			jobURL = fmt.Sprintf("https://startup.jobs/jobs/%v", r.ID)
		}
		post := model.JobPost{
			ID:          fmt.Sprintf("startupjobs-%v", r.ID),
			Title:       title,
			CompanyName: company,
			CompanyLogo: strings.TrimSpace(r.Company.Logo),
			JobURL:      jobURL,
			Location:    model.Location{City: strings.TrimSpace(r.Location)},
			Description: strings.TrimSpace(r.Description),
			IsRemote:    r.Remote,
			Skills:      r.Tags,
		}
		if t := parseStartupDate(firstNonEmpty(r.PublishedAt, r.CreatedAt)); t != nil {
			post.DatePosted = t
		}
		if post.ID == "startupjobs-<nil>" {
			post.ID = fmt.Sprintf("startupjobs-%d", i+1)
		}
		out = append(out, post)
	}
	if !util.HasMeaningfulJobs(out) {
		return nil, fmt.Errorf("startupjobs no parseable jobs")
	}
	return out, nil
}

type startupRow struct {
	ID          interface{} `json:"id"`
	Title       string      `json:"title"`
	CompanyName string      `json:"company_name"`
	Location    string      `json:"location"`
	Description string      `json:"description"`
	URL         string      `json:"url"`
	PublishedAt string      `json:"published_at"`
	CreatedAt   string      `json:"created_at"`
	Remote      bool        `json:"remote"`
	Tags        []string    `json:"tags"`
	Company     struct {
		Name string `json:"name"`
		Logo string `json:"logo"`
	} `json:"company"`
}

func (s *Scraper) fetch(ctx context.Context, endpoint string) ([]startupRow, error) {
	u, _ := url.Parse(endpoint)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("startupjobs request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("startupjobs status %d — try using --proxy with a residential proxy", resp.StatusCode)
	}

	body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("startupjobs read: %w", err)
	}

	var arr []startupRow
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&arr); err == nil {
		return arr, nil
	}
	var wrapped struct {
		Jobs  []startupRow `json:"jobs"`
		Items []startupRow `json:"items"`
	}
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&wrapped); err == nil {
		if len(wrapped.Jobs) > 0 {
			return wrapped.Jobs, nil
		}
		if len(wrapped.Items) > 0 {
			return wrapped.Items, nil
		}
	}
	return nil, nil
}

func parseStartupDate(v string) *time.Time {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	if t, err := time.Parse(time.RFC3339, v); err == nil {
		return &t
	}
	return nil
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
