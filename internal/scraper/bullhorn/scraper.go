package bullhorn

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/scraper/ats"
	"github.com/arinbalyan/scrappy/internal/util"
)

const defaultFields = "id,title,publicDescription,address,dateAdded,salary,salaryUnit,employmentType,categories"

// Scraper fetches jobs from Bullhorn Staffing.
// Slug format: {cls}:{corpToken}
type Scraper struct {
	client  *http.Client
	testURL string // if set, overrides the constructed URL (for tests)
}

// New creates a new Bullhorn scraper.
func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Timeout: 20 * time.Second})
	}
	return &Scraper{client: client}
}

// NewWithAPIURL creates a new Bullhorn scraper with a custom URL (used in tests).
func NewWithAPIURL(client *http.Client, apiURL string) *Scraper {
	s := New(client)
	if strings.TrimSpace(apiURL) != "" {
		s.testURL = apiURL
	}
	return s
}

func (s *Scraper) SiteName() model.Site { return model.SiteBullhorn }

// --- API response types ---

type bullhornAddress struct {
	City    string `json:"city"`
	State   string `json:"state"`
	Country string `json:"country"`
	Zip     string `json:"zip"`
}

type bullhornCategory struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type bullhornCategories struct {
	Data []bullhornCategory `json:"data"`
}

type bullhornJobOrder struct {
	ID                int                 `json:"id"`
	Title             string              `json:"title"`
	PublicDescription string              `json:"publicDescription"`
	Address           *bullhornAddress    `json:"address"`
	DateAdded         *int64              `json:"dateAdded"`
	Salary            *float64            `json:"salary"`
	SalaryUnit        string              `json:"salaryUnit"`
	EmploymentType    string              `json:"employmentType"`
	Categories        *bullhornCategories `json:"categories"`
}

type bullhornSearchResponse struct {
	Data  []bullhornJobOrder `json:"data"`
	Total int                `json:"total"`
	Count int                `json:"count"`
}

// Scrape fetches jobs from Bullhorn.
func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{"site": s.SiteName(), "results_wanted": input.ResultsWanted})

	seeds, src := ats.ResolveSeeds(input.SearchTerm, "SCRAPPY_BULLHORN_SEEDS")
	if len(seeds) == 0 {
		return nil, fmt.Errorf("bullhorn no seeds: set SCRAPPY_BULLHORN_SEEDS or pass a company slug in --search (format: cls:corpToken)")
	}
	util.Debug("bullhorn_seeds", map[string]any{"seeds": seeds, "src": src})

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

		cls, corpToken, err := parseSlug(slug)
		if err != nil {
			util.Warn("bullhorn_invalid_slug", map[string]any{"slug": slug, "err": err.Error()})
			continue
		}

		u := s.testURL
		if u == "" {
			u = fmt.Sprintf("https://public-rest%s.bullhornstaffing.com/rest-services/%s/search/JobOrder",
				url.PathEscape(cls), url.PathEscape(corpToken))
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			util.Warn("bullhorn_request_err", map[string]any{"slug": slug, "err": err.Error()})
			continue
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "Mozilla/5.0")

		q := req.URL.Query()
		q.Set("query", "(isOpen:1)")
		q.Set("fields", defaultFields)
		q.Set("count", fmt.Sprintf("%d", wanted))
		q.Set("start", "0")
		req.URL.RawQuery = q.Encode()

		resp, err := s.client.Do(req)
		if err != nil {
			util.Warn("bullhorn_fetch_fail", map[string]any{"slug": slug, "err": err.Error()})
			continue
		}

		body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
		resp.Body.Close()
		if err != nil {
			util.Warn("bullhorn_read_fail", map[string]any{"slug": slug, "err": err.Error()})
			continue
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			util.Warn("bullhorn_status", map[string]any{"slug": slug, "status": resp.StatusCode})
			continue
		}

		var searchResp bullhornSearchResponse
		if err := jsonUnmarshal(body, &searchResp); err != nil {
			util.Warn("bullhorn_decode_fail", map[string]any{"slug": slug, "err": err.Error()})
			continue
		}

		for _, order := range searchResp.Data {
			if len(out) >= wanted {
				break
			}

			title := strings.TrimSpace(order.Title)
			if title == "" {
				continue
			}

			id := ats.BuildID("bullhorn", slug, fmt.Sprintf("%d", order.ID))
			if seen[id] {
				continue
			}
			seen[id] = true

			// Location
			l := model.Location{}
			if order.Address != nil {
				l.City = strings.TrimSpace(order.Address.City)
				l.State = strings.TrimSpace(order.Address.State)
				l.Country = strings.TrimSpace(order.Address.Country)
			}

			// Department from categories
			dept := ""
			if order.Categories != nil && len(order.Categories.Data) > 0 {
				dept = strings.TrimSpace(order.Categories.Data[0].Name)
			}

			// Job URL
			jobURL := fmt.Sprintf("https://public-rest%s.bullhornstaffing.com/rest-services/%s/entity/JobOrder/%d",
				url.PathEscape(cls), url.PathEscape(corpToken), order.ID)

			jp := model.JobPost{
				ID:          id,
				Title:       title,
				CompanyName: corpToken,
				JobURL:      jobURL,
				Location:    l,
				Description: util.StripHTML(strings.TrimSpace(order.PublicDescription)),
				Site:        string(s.SiteName()),
				Department:  dept,
				JobType:     normalizeEmploymentType(order.EmploymentType),
			}

			// Compensation
			if order.Salary != nil && *order.Salary != 0 {
				jp.Compensation = &model.Compensation{
					Interval:  mapSalaryUnit(order.SalaryUnit),
					MinAmount: order.Salary,
					Currency:  "USD",
				}
			}

			// Date posted (epoch ms)
			if order.DateAdded != nil && *order.DateAdded > 0 {
				t := time.UnixMilli(*order.DateAdded)
				jp.DatePosted = &t
			}

			out = append(out, jp)
		}
	}

	if !util.HasMeaningfulJobs(out) {
		return nil, fmt.Errorf("bullhorn no parseable jobs")
	}
	return out, nil
}

func parseSlug(slug string) (cls, corpToken string, err error) {
	idx := strings.Index(slug, ":")
	if idx == -1 {
		return "", "", fmt.Errorf("invalid bullhorn slug %q: expected cls:corpToken format", slug)
	}
	cls = slug[:idx]
	corpToken = slug[idx+1:]
	if cls == "" || corpToken == "" {
		return "", "", fmt.Errorf("invalid bullhorn slug %q: cls and corpToken must not be empty", slug)
	}
	return cls, corpToken, nil
}

func mapSalaryUnit(unit string) model.CompensationInterval {
	switch strings.ToLower(strings.TrimSpace(unit)) {
	case "per hour", "hourly", "hour":
		return model.IntervalHourly
	case "per day", "daily", "day":
		return model.IntervalDaily
	case "per week", "weekly", "week":
		return model.IntervalWeekly
	case "per month", "monthly", "month":
		return model.IntervalMonthly
	default:
		return model.IntervalYearly
	}
}

func normalizeEmploymentType(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "fulltime", "full-time", "full time":
		return "fulltime"
	case "parttime", "part-time", "part time":
		return "parttime"
	case "contract", "contractor":
		return "contract"
	case "internship", "intern":
		return "internship"
	}
	return v
}

func jsonUnmarshal(data []byte, v any) error {
	return json.Unmarshal(data, v)
}
