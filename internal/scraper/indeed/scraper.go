package indeed

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/util"
)

const apiURL = "https://apis.indeed.com/graphql"

var indeedJobKeyRe = regexp.MustCompile(`"jobKey":"([a-zA-Z0-9_-]+)"`)

const jobSearchQueryTemplate = `query GetJobData {
  jobSearch(
    %s
    %s
    limit: 100
    %s
    sort: RELEVANCE
    %s
  ) {
    pageInfo { nextCursor }
    results {
      job {
        key
        title
        datePublished
        description { html }
        location {
          countryCode
          admin1Code
          city
          formatted { long }
        }
        attributes { key label }
        compensation {
          estimated {
            currencyCode
            baseSalary { unitOfWork range { ... on Range { min max } } }
          }
          baseSalary { unitOfWork range { ... on Range { min max } } }
          currencyCode
        }
        employer {
          name
          relativeCompanyPageUrl
          dossier {
            employerDetails {
              addresses
              industry
              employeesLocalizedLabel
              revenueLocalizedLabel
              briefDescription
            }
            images { squareLogoUrl }
            links { corporateWebsite }
          }
        }
        recruit { viewJobUrl }
      }
    }
  }
}`

type Scraper struct {
	client *http.Client
	apiURL string
}

func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Retries: 3, CookieResetEveryN: 120, Timeout: 15 * time.Second})
	}
	return &Scraper{client: client, apiURL: apiURL}
}

func NewWithAPIURL(client *http.Client, endpoint string) *Scraper {
	s := New(client)
	if strings.TrimSpace(endpoint) != "" {
		s.apiURL = endpoint
	}
	return s
}

func (s *Scraper) SiteName() model.Site { return model.SiteIndeed }

func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	if input.ResultsWanted <= 0 {
		input.ResultsWanted = 15
	}

	jobs := make([]model.JobPost, 0, input.ResultsWanted)
	seen := map[string]struct{}{}
	var cursor string

	for len(jobs) < input.Offset+input.ResultsWanted {
		pageJobs, nextCursor, err := s.scrapePage(ctx, input, cursor, seen)
		if err != nil {
			return nil, err
		}
		if len(pageJobs) == 0 {
			break
		}
		jobs = append(jobs, pageJobs...)
		if nextCursor == "" || nextCursor == cursor {
			break
		}
		cursor = nextCursor
	}

	start := input.Offset
	if start > len(jobs) {
		start = len(jobs)
	}
	end := start + input.ResultsWanted
	if end > len(jobs) {
		end = len(jobs)
	}
	return jobs[start:end], nil
}

func (s *Scraper) scrapePage(ctx context.Context, input model.ScraperInput, cursor string, seen map[string]struct{}) ([]model.JobPost, string, error) {
	query := buildQuery(input, cursor)
	payload := map[string]string{"query": query}
	b, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.apiURL, bytes.NewReader(b))
	if err != nil {
		return nil, "", fmt.Errorf("create indeed request: %w", err)
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("accept", "application/json")
	req.Header.Set("accept-language", "en-US,en;q=0.9")
	req.Header.Set("origin", "https://www.indeed.com")
	req.Header.Set("referer", "https://www.indeed.com/")
	req.Header.Set("indeed-locale", "en-US")
	req.Header.Set("user-agent", "Mozilla/5.0")
	req.Header.Set("indeed-app-info", "appv=193.1; appid=com.indeed.jobsearch; osv=16.6.1; os=ios; dtype=phone")
	if host := req.URL.Host; host != "" {
		req.Header.Set("host", host)
	}
	if v := strings.TrimSpace(os.Getenv("SCRAPPY_INDEED_API_KEY")); v != "" {
		req.Header.Set("indeed-api-key", v)
	}
	if v := strings.TrimSpace(os.Getenv("SCRAPPY_INDEED_CO")); v != "" {
		req.Header.Set("indeed-co", v)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("execute indeed request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		if fallbackJobs, fallbackErr := s.scrapeHTMLFallback(ctx, input, seen); fallbackErr == nil {
			return fallbackJobs, "", nil
		}
		return nil, "", nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("indeed api status %d", resp.StatusCode)
	}

	var parsed graphQLResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, "", fmt.Errorf("decode indeed response: %w", err)
	}

	out := make([]model.JobPost, 0, len(parsed.Data.JobSearch.Results))
	for _, item := range parsed.Data.JobSearch.Results {
		job := item.Job
		jobURL := "https://www.indeed.com/viewjob?jk=" + job.Key
		if _, ok := seen[jobURL]; ok {
			continue
		}
		seen[jobURL] = struct{}{}
		out = append(out, toJobPost(job, jobURL))
	}

	return out, parsed.Data.JobSearch.PageInfo.NextCursor, nil
}

func buildQuery(input model.ScraperInput, cursor string) string {
	what := ""
	if strings.TrimSpace(input.SearchTerm) != "" {
		what = fmt.Sprintf("what: %q", strings.TrimSpace(input.SearchTerm))
	}

	location := ""
	if strings.TrimSpace(input.Location) != "" {
		r := input.DistanceMiles
		if r <= 0 {
			r = 50
		}
		location = fmt.Sprintf("location: {where: %q, radius: %d, radiusUnit: MILES}", strings.TrimSpace(input.Location), r)
	}

	cursorClause := ""
	if cursor != "" {
		cursorClause = fmt.Sprintf("cursor: %q", cursor)
	}

	filters := buildFilters(input)
	return fmt.Sprintf(jobSearchQueryTemplate, what, location, cursorClause, filters)
}

func buildFilters(input model.ScraperInput) string {
	if input.HoursOld > 0 {
		return fmt.Sprintf("filters: { date: { field: \"dateOnIndeed\", start: \"%dh\" } }", input.HoursOld)
	}
	if input.EasyApply {
		return "filters: { keyword: { field: \"indeedApplyScope\", keys: [\"DESKTOP\"] } }"
	}

	keys := make([]string, 0, 2)
	switch input.JobType {
	case model.JobTypeFullTime:
		keys = append(keys, "CF3CP")
	case model.JobTypePartTime:
		keys = append(keys, "75GKK")
	case model.JobTypeContract:
		keys = append(keys, "NJXCK")
	case model.JobTypeInternship:
		keys = append(keys, "VDTG7")
	}
	if input.IsRemote {
		keys = append(keys, "DSQF7")
	}
	if len(keys) == 0 {
		return ""
	}
	quoted := make([]string, 0, len(keys))
	for _, k := range keys {
		quoted = append(quoted, fmt.Sprintf("\"%s\"", k))
	}
	return "filters: { composite: { filters: [{ keyword: { field: \"attributes\", keys: [" + strings.Join(quoted, ",") + "] } }] } }"
}

func toJobPost(j indeedJob, jobURL string) model.JobPost {
	var posted *time.Time
	if j.DatePublished > 0 {
		t := time.UnixMilli(j.DatePublished)
		posted = &t
	}

	jp := model.JobPost{
		ID:          "in-" + j.Key,
		Title:       j.Title,
		Description: j.Description.HTML,
		CompanyName: j.Employer.Name,
		JobURL:      jobURL,
		JobURLDirect: j.Recruit.ViewJobURL,
		Location: model.Location{
			City:    j.Location.City,
			State:   j.Location.Admin1Code,
			Country: j.Location.CountryCode,
		},
		DatePosted:           posted,
		IsRemote:             hasRemoteSignal(j),
		CompanyIndustry:      normalizeIndustry(j.Employer.Dossier.EmployerDetails.Industry),
		CompanyNumEmployees:  j.Employer.Dossier.EmployerDetails.EmployeesLocalizedLabel,
		CompanyRevenue:       j.Employer.Dossier.EmployerDetails.RevenueLocalizedLabel,
		CompanyDescription:   j.Employer.Dossier.EmployerDetails.BriefDescription,
		CompanyLogo:          j.Employer.Dossier.Images.SquareLogoURL,
		CompanyURL:           "https://www.indeed.com" + j.Employer.RelativeCompanyPageURL,
		CompanyAddresses:     firstOrEmpty(j.Employer.Dossier.EmployerDetails.Addresses),
	}
	if c := parseCompensation(j.Compensation); c != nil {
		jp.Compensation = c
	}
	return jp
}

func hasRemoteSignal(j indeedJob) bool {
	for _, a := range j.Attributes {
		v := strings.ToLower(a.Label)
		if strings.Contains(v, "remote") || strings.Contains(v, "work from home") || strings.Contains(v, "wfh") {
			return true
		}
	}
	if strings.Contains(strings.ToLower(j.Location.Formatted.Long), "remote") {
		return true
	}
	if strings.Contains(strings.ToLower(j.Description.HTML), "remote") {
		return true
	}
	return false
}

func firstOrEmpty(vals []string) string {
	if len(vals) == 0 {
		return ""
	}
	return vals[0]
}

func normalizeIndustry(v string) string {
	v = strings.ReplaceAll(v, "Iv1", "")
	v = strings.ReplaceAll(v, "_", " ")
	return strings.TrimSpace(strings.Title(strings.ToLower(v)))
}

func parseCompensation(c indeedCompensation) *model.Compensation {
	var base *indeedBaseSalary
	currency := c.CurrencyCode
	if c.BaseSalary != nil {
		base = c.BaseSalary
	}
	if base == nil && c.Estimated != nil && c.Estimated.BaseSalary != nil {
		base = c.Estimated.BaseSalary
		if c.Estimated.CurrencyCode != "" {
			currency = c.Estimated.CurrencyCode
		}
	}
	if base == nil || base.Range == nil {
		return nil
	}

	interval, ok := mapUnitToInterval(base.UnitOfWork)
	if !ok {
		return nil
	}
	comp := &model.Compensation{Interval: interval, Currency: currency}
	if base.Range.Min != nil {
		v := *base.Range.Min
		comp.MinAmount = &v
	}
	if base.Range.Max != nil {
		v := *base.Range.Max
		comp.MaxAmount = &v
	}
	return comp
}

func mapUnitToInterval(unit string) (model.CompensationInterval, bool) {
	switch strings.ToUpper(strings.TrimSpace(unit)) {
	case "YEAR":
		return model.IntervalYearly, true
	case "MONTH":
		return model.IntervalMonthly, true
	case "WEEK":
		return model.IntervalWeekly, true
	case "DAY":
		return model.IntervalDaily, true
	case "HOUR":
		return model.IntervalHourly, true
	default:
		return "", false
	}
}

func (s *Scraper) scrapeHTMLFallback(ctx context.Context, input model.ScraperInput, seen map[string]struct{}) ([]model.JobPost, error) {
	searchURL := buildIndeedSearchURL(input, s.apiURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create indeed fallback request: %w", err)
	}
	req.Header.Set("accept", "text/html,application/xhtml+xml")
	req.Header.Set("accept-language", "en-US,en;q=0.9")
	req.Header.Set("user-agent", "Mozilla/5.0")
	req.Header.Set("referer", "https://www.indeed.com/")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute indeed fallback request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("indeed fallback status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read indeed fallback body: %w", err)
	}

	keys := parseIndeedJobKeys(string(body))
	if len(keys) == 0 {
		return nil, nil
	}
	out := make([]model.JobPost, 0, len(keys))
	for i, key := range keys {
		jobURL := "https://www.indeed.com/viewjob?jk=" + key
		if _, ok := seen[jobURL]; ok {
			continue
		}
		seen[jobURL] = struct{}{}
		out = append(out, model.JobPost{ID: "in-" + key, JobURL: jobURL, Title: "(fallback) Indeed listing", CompanyName: "", IsRemote: input.IsRemote})
		if len(out) >= input.ResultsWanted && input.ResultsWanted > 0 {
			break
		}
		_ = i
	}
	return out, nil
}

func buildIndeedSearchURL(input model.ScraperInput, apiEndpoint string) string {
	base := "https://www.indeed.com/jobs"
	if u, err := url.Parse(strings.TrimSpace(apiEndpoint)); err == nil && u != nil && u.Scheme != "" && u.Host != "" {
		base = u.Scheme + "://" + u.Host + "/jobs"
	}
	u, _ := url.Parse(base)
	q := u.Query()
	if strings.TrimSpace(input.SearchTerm) != "" {
		q.Set("q", strings.TrimSpace(input.SearchTerm))
	}
	if strings.TrimSpace(input.Location) != "" {
		q.Set("l", strings.TrimSpace(input.Location))
	}
	if input.Offset > 0 {
		q.Set("start", strconv.Itoa(input.Offset))
	}
	u.RawQuery = q.Encode()
	return u.String()
}

func parseIndeedJobKeys(html string) []string {
	m := indeedJobKeyRe.FindAllStringSubmatch(html, -1)
	if len(m) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	keys := make([]string, 0, len(m))
	for _, row := range m {
		if len(row) < 2 {
			continue
		}
		k := strings.TrimSpace(row[1])
		if k == "" {
			continue
		}
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		keys = append(keys, k)
	}
	return keys
}

type graphQLResponse struct {
	Data struct {
		JobSearch struct {
			PageInfo struct {
				NextCursor string `json:"nextCursor"`
			} `json:"pageInfo"`
			Results []struct {
				Job indeedJob `json:"job"`
			} `json:"results"`
		} `json:"jobSearch"`
	} `json:"data"`
}

type indeedJob struct {
	Key           string `json:"key"`
	Title         string `json:"title"`
	DatePublished int64  `json:"datePublished"`
	Description   struct {
		HTML string `json:"html"`
	} `json:"description"`
	Location struct {
		CountryCode string `json:"countryCode"`
		Admin1Code  string `json:"admin1Code"`
		City        string `json:"city"`
		Formatted   struct {
			Long string `json:"long"`
		} `json:"formatted"`
	} `json:"location"`
	Attributes []struct {
		Label string `json:"label"`
	} `json:"attributes"`
	Compensation indeedCompensation `json:"compensation"`
	Employer     struct {
		Name                   string `json:"name"`
		RelativeCompanyPageURL string `json:"relativeCompanyPageUrl"`
		Dossier                struct {
			EmployerDetails struct {
				Addresses              []string `json:"addresses"`
				Industry               string   `json:"industry"`
				EmployeesLocalizedLabel string  `json:"employeesLocalizedLabel"`
				RevenueLocalizedLabel  string   `json:"revenueLocalizedLabel"`
				BriefDescription       string   `json:"briefDescription"`
			} `json:"employerDetails"`
			Images struct {
				SquareLogoURL string `json:"squareLogoUrl"`
			} `json:"images"`
		} `json:"dossier"`
	} `json:"employer"`
	Recruit struct {
		ViewJobURL string `json:"viewJobUrl"`
	} `json:"recruit"`
}

type indeedCompensation struct {
	Estimated *struct {
		CurrencyCode string            `json:"currencyCode"`
		BaseSalary   *indeedBaseSalary `json:"baseSalary"`
	} `json:"estimated"`
	BaseSalary   *indeedBaseSalary `json:"baseSalary"`
	CurrencyCode string            `json:"currencyCode"`
}

type indeedBaseSalary struct {
	UnitOfWork string `json:"unitOfWork"`
	Range      *struct {
		Min *float64 `json:"min"`
		Max *float64 `json:"max"`
	} `json:"range"`
}
