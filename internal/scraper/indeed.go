package scraper

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/arinbalyan/scrappy/internal/config"
	"github.com/arinbalyan/scrappy/internal/types"
)

// ─── Indeed ──────────────────────────────────────────────────────────

type indeedScraper struct {
	id       string
	apiKey   string
	search   string
	location string
	results  int
	country  string
	isRemote *bool
	jobType  string
	hoursOld *int
	client   *http.Client
}

func NewIndeed(s config.Site) *indeedScraper {
	r := 50
	if s.Results > 0 {
		r = s.Results
	}
	key := s.IndeedAPIKey
	if key == "" {
		key = strings.TrimSpace(os.Getenv("SCRAPPY_INDEED_API_KEY"))
	}
	return &indeedScraper{
		id:       s.ID,
		apiKey:   key,
		search:   s.Search,
		location: s.Location,
		results:  r,
		country:  strings.ToUpper(s.Country),
		isRemote: s.IsRemote,
		jobType:  s.JobType,
		hoursOld: s.HoursOld,
		client:   &http.Client{Timeout: 30 * time.Second},
	}
}

func runIndeed(s config.Site) func(context.Context, string) ([]types.JobPosting, error) {
	sc := NewIndeed(s)
	return func(ctx context.Context, query string) ([]types.JobPosting, error) {
		return sc.Scrape(ctx, query)
	}
}

func (e *indeedScraper) Name() string { return e.id }

func (e *indeedScraper) Scrape(ctx context.Context, query string) ([]types.JobPosting, error) {
	if query != "" && e.search == "" {
		e.search = query
	}
	if e.apiKey == "" {
		return nil, fmt.Errorf("indeed: set api_key in config or SCRAPPY_INDEED_API_KEY env")
	}

	// Warm up session — Indeed requires cookies from website before GraphQL works
	_ = e.warmupSession(ctx)

	country := e.country
	if country == "" {
		country = "USA"
	}
	indeedCo := indeedCountryCode(country)
	if v := strings.TrimSpace(os.Getenv("SCRAPPY_INDEED_CO")); v != "" {
		indeedCo = strings.ToUpper(v)
	}

	var jobs []types.JobPosting
	cursor := ""

	for len(jobs) < e.results {
		select {
		case <-ctx.Done():
			return jobs, ctx.Err()
		default:
		}

		q := buildIndeedQuery(e.search, e.location, e.hoursOld, e.isRemote, e.jobType, cursor)
		body := fmt.Sprintf(`{"query": %q}`, q)

		req, err := http.NewRequestWithContext(ctx, "POST", "https://apis.indeed.com/graphql", strings.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Host", "apis.indeed.com")
		req.Header.Set("content-type", "application/json")
		req.Header.Set("accept", "application/json")
		req.Header.Set("indeed-locale", "en-US")
		req.Header.Set("accept-language", "en-US,en;q=0.9")
		req.Header.Set("user-agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 16_6_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Mobile/15E148 Indeed App 193.1")
		req.Header.Set("indeed-app-info", "appv=193.1; appid=com.indeed.jobsearch; osv=16.6.1; os=ios; dtype=phone")
		req.Header.Set("indeed-api-key", e.apiKey)
		req.Header.Set("indeed-co", indeedCo)

		resp, err := e.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("indeed request: %w", err)
		}
		b, _ := ioReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return jobs, fmt.Errorf("indeed HTTP %d: %s", resp.StatusCode, truncate(string(b), 200))
		}

		pageJobs, nextCursor, err := parseIndeedResponse(b, e.id)
		if err != nil {
			return nil, fmt.Errorf("indeed parse: %w", err)
		}

		for _, j := range pageJobs {
			seen := false
			for _, s := range jobs {
				if s.URL == j.URL {
					seen = true
					break
				}
			}
			if !seen {
				j.Source = e.id
				jobs = append(jobs, j)
			}
		}

		if nextCursor == "" || len(pageJobs) == 0 {
			break
		}
		cursor = nextCursor
	}

	if len(jobs) > e.results {
		jobs = jobs[:e.results]
	}
	return jobs, nil
}

// ─── Indeed GraphQL query builder ────────────────────────────────────

func buildIndeedQuery(search, location string, hoursOld *int, isRemote *bool, jobType, cursor string) string {
	var sb strings.Builder
	if search != "" {
		sb.WriteString(fmt.Sprintf("what: %q\n", search))
	}
	if location != "" {
		sb.WriteString(fmt.Sprintf("where: %q\n", location))
	}
	sb.WriteString("limit: 100\n")
	if cursor != "" {
		sb.WriteString(fmt.Sprintf("cursor: %q\n", cursor))
	}
	if hoursOld != nil {
		sb.WriteString(fmt.Sprintf("dateOnIndeed: %d\n", *hoursOld))
	} else if isRemote != nil || jobType != "" {
		var keys []string
		if isRemote != nil && *isRemote {
			keys = append(keys, "DSQF7")
		}
		switch strings.ToLower(jobType) {
		case "fulltime", "full-time":
			keys = append(keys, "CF3CP")
		case "parttime", "part-time":
			keys = append(keys, "NB6M6")
		case "contract":
			keys = append(keys, "D36H")
		case "internship":
			keys = append(keys, "H397")
		}
		if len(keys) > 0 {
			sb.WriteString("filters: {\n  composite: {\n    filters: [\n")
			sb.WriteString("  {keyword: {field: \"attributes\", keys: [\"")
			sb.WriteString(strings.Join(keys, "\", \""))
			sb.WriteString("\"]}}\n  ]\n  }\n}\n")
		}
	}
	return fmt.Sprintf(`query GetJobData {{
jobSearch(
%s
sort: RELEVANCE
) {
pageInfo { nextCursor }
results {
  job {
    source { name }
    key
    title
    datePublished
    dateOnIndeed
    description { html }
    location {
      formatted { short long }
    }
    compensation {
      estimated {
        currencyCode
        baseSalary {
          unitOfWork
          range { ... on Range { min max } }
        }
      }
      baseSalary {
        unitOfWork
        range { ... on Range { min max } }
      }
      currencyCode
    }
    attributes { key label }
    employer {
      relativeCompanyPageUrl
      name
      dossier {
        employerDetails {
          industry
          employeesLocalizedLabel
          revenueLocalizedLabel
          briefDescription
          ceoName
        }
        links { corporateWebsite }
      }
    }
    recruit {
      viewJobUrl
      detailedSalary
      workSchedule
    }
  }
}}
}}`, sb.String())
}

// ─── Indeed JSON parser ──────────────────────────────────────────────

func parseIndeedResponse(b []byte, source string) ([]types.JobPosting, string, error) {
	var raw struct {
		Data struct {
			JobSearch struct {
				PageInfo struct {
					NextCursor string `json:"nextCursor"`
				}
				Results []struct {
					Job []byte `json:"job"`
				} `json:"results"`
			} `json:"jobSearch"`
		} `json:"data"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, "", err
	}
	jobs := make([]types.JobPosting, 0, len(raw.Data.JobSearch.Results))
	for _, r := range raw.Data.JobSearch.Results {
		jobs = append(jobs, decodeIndeedJob(r.Job, source))
	}
	return jobs, raw.Data.JobSearch.PageInfo.NextCursor, nil
}

func decodeIndeedJob(raw []byte, source string) types.JobPosting {
	var v map[string]interface{}
	json.Unmarshal(raw, &v)
	j := types.JobPosting{Source: source}
	j.ID = gs(v, "key")
	j.Title = gs(v, "title")
	j.Description = gn(v, "description", "html")
	j.Salary = gs(v, "detailedSalary")

	if loc, ok := v["location"].(map[string]interface{}); ok {
		if f, ok := loc["formatted"].(map[string]interface{}); ok {
			j.Location = gs(f, "long")
			if j.Location == "" {
				j.Location = gs(f, "short")
			}
		}
	}
	if comp, ok := v["compensation"].(map[string]interface{}); ok {
		if est, ok := comp["estimated"].(map[string]interface{}); ok {
			if bs, ok := est["baseSalary"].(map[string]interface{}); ok {
				if rng, ok := bs["range"].(map[string]interface{}); ok {
					j.SalaryMin = gf(rng, "min")
					j.SalaryMax = gf(rng, "max")
					j.SalaryPeriod = gs(bs, "unitOfWork")
					j.Currency = gs(est, "currencyCode")
				}
			}
		}
	}
	if emp, ok := v["employer"].(map[string]interface{}); ok {
		j.Company = gs(emp, "name")
		if d, ok := emp["dossier"].(map[string]interface{}); ok {
			if ed, ok := d["employerDetails"].(map[string]interface{}); ok {
				j.Industry = gs(ed, "industry")
			}
			if lnks, ok := d["links"].(map[string]interface{}); ok {
				j.CompanyURL = gs(lnks, "corporateWebsite")
			}
		}
	}
	if rec, ok := v["recruit"].(map[string]interface{}); ok {
		j.URL = gs(rec, "viewJobUrl")
	}
	if dt, err := time.Parse(time.RFC3339, gs(v, "datePublished")); err == nil {
		j.PostedAt = &dt
	}
	if attrs, ok := v["attributes"].([]interface{}); ok {
		for _, a := range attrs {
			if m, ok := a.(map[string]interface{}); ok {
				if gs(m, "key") == "jobType" {
					j.JobType = gs(m, "label")
				}
			}
		}
	}
	return j
}

// ─── Generic mechanism stubs ─────────────────────────────────────────

func runHTML(s config.Site)       func(context.Context, string) ([]types.JobPosting, error) { return func(ctx context.Context, query string) ([]types.JobPosting, error) { return nil, fmt.Errorf("html not yet implemented for %s", s.ID) } }
func runAPI(s config.Site)        func(context.Context, string) ([]types.JobPosting, error) { return func(ctx context.Context, query string) ([]types.JobPosting, error) { return nil, fmt.Errorf("api not yet implemented for %s", s.ID) } }
func runGraphQL(s config.Site)    func(context.Context, string) ([]types.JobPosting, error) { return func(ctx context.Context, query string) ([]types.JobPosting, error) { return nil, fmt.Errorf("graphql not yet implemented for %s", s.ID) } }
func runRSS(s config.Site)        func(context.Context, string) ([]types.JobPosting, error) { return func(ctx context.Context, query string) ([]types.JobPosting, error) { return nil, fmt.Errorf("rss not yet implemented for %s", s.ID) } }

// ─── Helpers ─────────────────────────────────────────────────────────

func ioReadAll(r io.Reader) ([]byte, error) { return io.ReadAll(r) }
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
func gs(m map[string]interface{}, k string) string {
	if v, ok := m[k].(string); ok {
		return v
	}
	return ""
}
func gn(m map[string]interface{}, k1, k2 string) string {
	if a, ok := m[k1].(map[string]interface{}); ok {
		return gs(a, k2)
	}
	return ""
}
func gf(m map[string]interface{}, k string) *float64 {
	if v, ok := m[k]; ok {
		if f, ok := v.(float64); ok {
			return &f
		}
	}
	return nil
}

// indeedCountryCode maps full country names to the 2-letter codes Indeed expects.
func indeedCountryCode(country string) string {
	switch strings.ToUpper(country) {
	case "CA", "CANADA":
		return "CA"
	case "GB", "UK", "UNITED KINGDOM":
		return "GB"
	case "DE", "GERMANY":
		return "DE"
	case "FR", "FRANCE":
		return "FR"
	case "IN", "INDIA":
		return "IN"
	case "AU", "AUSTRALIA":
		return "AU"
	case "US", "USA", "UNITED STATES":
		fallthrough
	default:
		return "US"
	}
}

// warmupSession hits Indeed website pages to set cookies — required before GraphQL API works.
func (e *indeedScraper) warmupSession(ctx context.Context) error {
	host := "www.indeed.com"
	ua := "Mozilla/5.0 (iPhone; CPU iPhone OS 16_6_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Mobile/15E148 Indeed App 193.1"

	urls := []string{
		"https://" + host + "/",
		"https://" + host + "/jobs",
	}

	for _, u := range urls {
		req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
		if err != nil {
			continue
		}
		req.Header.Set("accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
		req.Header.Set("accept-language", "en-US,en;q=0.9")
		req.Header.Set("user-agent", ua)

		resp, err := e.client.Do(req)
		if err != nil {
			continue
		}
		resp.Body.Close()
	}
	return nil
}
