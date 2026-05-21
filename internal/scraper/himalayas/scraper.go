package himalayas

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/util"
)

const defaultAPI = "https://himalayas.app/jobs/api/search"

type Scraper struct {
	client *http.Client
	apiURL string
}

func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Retries: 3, CookieResetEveryN: 120, Timeout: 18 * time.Second})
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

func (s *Scraper) SiteName() model.Site { return model.SiteHimalayas }

func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	u, _ := url.Parse(s.apiURL)
	q := u.Query()
	term := strings.TrimSpace(input.SearchTerm)
	if term != "" {
		q.Set("q", term)
	}
	q.Set("page", "1")
	u.RawQuery = q.Encode()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	req.Header.Set("Accept", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("himalayas request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("himalayas status %d", resp.StatusCode)
	}

	var parsed struct {
		Jobs []struct {
			Guid                 string      `json:"guid"`
			Title                string      `json:"title"`
			Excerpt              string      `json:"excerpt"`
			CompanyName          string      `json:"companyName"`
			ApplicationLink      string      `json:"applicationLink"`
			PubDate              any         `json:"pubDate"`
			EmploymentType       string      `json:"employmentType"`
			Categories           []string    `json:"categories"`
			LocationRestrictions interface{} `json:"locationRestrictions"`
		} `json:"jobs"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("himalayas decode: %w", err)
	}

	limit := input.ResultsWanted
	if limit <= 0 || limit > len(parsed.Jobs) {
		limit = len(parsed.Jobs)
	}
	out := make([]model.JobPost, 0, limit)
	for i := 0; i < limit; i++ {
		r := parsed.Jobs[i]
		loc := firstLocationRestriction(r.LocationRestrictions)
		post := model.JobPost{
			ID:          fallbackID("hm-", r.Guid, r.Title, i),
			Title:       strings.TrimSpace(r.Title),
			CompanyName: strings.TrimSpace(r.CompanyName),
			Description: strings.TrimSpace(r.Excerpt),
			JobURL:      strings.TrimSpace(r.ApplicationLink),
			Location:    model.Location{City: loc},
			IsRemote:    true,
			JobType:     strings.ToLower(strings.TrimSpace(r.EmploymentType)),
		}
		if post.Description == "" && len(r.Categories) > 0 {
			post.Description = strings.Join(r.Categories, ", ")
		}
		if post.JobURL == "" {
			post.JobURL = "https://himalayas.app/jobs"
		}
		post.DatePosted = parseUnixOrRFC3339(r.PubDate)
		out = append(out, post)
	}
	if !util.HasMeaningfulJobs(out) {
		return nil, fmt.Errorf("himalayas no parseable jobs")
	}
	return out, nil
}

func fallbackID(prefix, guid, title string, i int) string {
	guid = strings.TrimSpace(guid)
	if guid != "" {
		return prefix + guid
	}
	return fmt.Sprintf("%s%s-%d", prefix, util.NormalizeSlug(title), i+1)
}

func parseUnixOrRFC3339(v any) *time.Time {
	switch t := v.(type) {
	case float64:
		ts := time.UnixMilli(int64(t))
		return &ts
	case string:
		t = strings.TrimSpace(t)
		if t == "" {
			return nil
		}
		if n, err := strconv.ParseInt(t, 10, 64); err == nil {
			ts := time.UnixMilli(n)
			return &ts
		}
		if ts, err := time.Parse(time.RFC3339, t); err == nil {
			return &ts
		}
	}
	return nil
}

func firstLocationRestriction(v interface{}) string {
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case []interface{}:
		for _, item := range t {
			switch x := item.(type) {
			case string:
				x = strings.TrimSpace(x)
				if x != "" {
					return x
				}
			case map[string]interface{}:
				if n, ok := x["name"].(string); ok {
					n = strings.TrimSpace(n)
					if n != "" {
						return n
					}
				}
			}
		}
	}
	return ""
}
