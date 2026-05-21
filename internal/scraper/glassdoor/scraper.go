package glassdoor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/util"
)

const (
	defaultURL        = "https://www.glassdoor.com/Job/jobs.htm"
	defaultGraphURL   = "https://www.glassdoor.com/graph"
	defaultLocationID = 11047
)

var (
	reJob        = regexp.MustCompile(`(?s)data-jobid="([^"]+)"[\s\S]*?<a[^>]*class="jobLink"[^>]*>([^<]+)</a>[\s\S]*?<span[^>]*class="EmployerProfile_compactEmployerName"[^>]*>([^<]+)</span>`)
	reLDScript   = regexp.MustCompile(`(?s)<script[^>]*type="application/ld\+json"[^>]*>(.*?)</script>`)
	reToken      = regexp.MustCompile(`"token"\s*:\s*"([^"]+)"`)
	fallbackCSRF = "**********************:0pGUrkb2y3TyOpAIqF2vbPmUXoXVkD3oEGDVkvfeCerceQ5-n8mBg3BovySUIjmCPHCaW0H2nQVdqzbtsYqf4Q:wcqRqeegRUa9MVLJGyujVXB7vWFPjdaS1CtrrzJq-ok"
)

type Scraper struct {
	client      *http.Client
	listURL     string
	graphURL    string
	warmupOnce  sync.Once
	warmupError error
	csrfToken   string
}

func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Retries: 2, CookieResetEveryN: 100})
	}
	return &Scraper{client: client, listURL: defaultURL, graphURL: defaultGraphURL}
}
func NewWithListURL(client *http.Client, u string) *Scraper {
	s := New(client)
	if strings.TrimSpace(u) != "" {
		s.listURL = u
	}
	return s
}
func (s *Scraper) SiteName() model.Site { return model.SiteGlassdoor }

func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{"site": s.SiteName(), "results_wanted": input.ResultsWanted, "search_term": input.SearchTerm, "location": input.Location})
	s.warmupOnce.Do(func() { s.warmupError = s.bootstrapSession(ctx) })

	if jobs, err := s.scrapeGraphQL(ctx, input); err == nil && len(jobs) > 0 {
		return limitJobs(jobs, input.ResultsWanted), nil
	}

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, s.listURL, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("glassdoor request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusForbidden {
		return nil, fmt.Errorf("glassdoor blocked status 403")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("glassdoor status %d", resp.StatusCode)
	}
	b, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("glassdoor read: %w", err)
	}
	raw := string(b)
	if strings.Contains(strings.ToLower(raw), "security | glassdoor") || strings.Contains(strings.ToLower(raw), "just a moment") {
		return nil, fmt.Errorf("glassdoor challenge page")
	}

	if jobs := parseLDJSONJobs(raw); len(jobs) > 0 {
		limited := limitJobs(jobs, input.ResultsWanted)
		if util.HasMeaningfulJobs(limited) {
			return limited, nil
		}
	}
	limited := limitJobs(parseHTMLJobs(raw, s.listURL), input.ResultsWanted)
	if !util.HasMeaningfulJobs(limited) {
		return nil, fmt.Errorf("glassdoor no parseable jobs")
	}
	return limited, nil
}

func limitJobs(in []model.JobPost, wanted int) []model.JobPost {
	if wanted <= 0 || wanted > len(in) {
		return in
	}
	return in[:wanted]
}

func parseHTMLJobs(raw, sourceURL string) []model.JobPost {
	m := reJob.FindAllStringSubmatch(raw, -1)
	out := make([]model.JobPost, 0, len(m))
	for _, row := range m {
		out = append(out, model.JobPost{
			ID:          "gd-" + row[1],
			Title:       strings.TrimSpace(row[2]),
			CompanyName: strings.TrimSpace(row[3]),
			JobURL:      sourceURL,
		})
	}
	return out
}

func parseLDJSONJobs(raw string) []model.JobPost {
	type ldJob struct {
		Type        string `json:"@type"`
		Title       string `json:"title"`
		Description string `json:"description"`
		DatePosted  string `json:"datePosted"`
		URL         string `json:"url"`
		HiringOrg   struct {
			Name string `json:"name"`
		} `json:"hiringOrganization"`
	}
	type ldGraph struct {
		Graph []ldJob `json:"@graph"`
	}

	scripts := reLDScript.FindAllStringSubmatch(raw, -1)
	jobs := make([]model.JobPost, 0)
	for i, s := range scripts {
		body := strings.TrimSpace(s[1])
		if body == "" {
			continue
		}
		var single ldJob
		if err := json.Unmarshal([]byte(body), &single); err == nil && strings.EqualFold(single.Type, "JobPosting") {
			jobs = append(jobs, toPost(single.Title, single.HiringOrg.Name, single.Description, single.URL, single.DatePosted, i))
			continue
		}
		var many []ldJob
		if err := json.Unmarshal([]byte(body), &many); err == nil {
			for idx, j := range many {
				if strings.EqualFold(j.Type, "JobPosting") {
					jobs = append(jobs, toPost(j.Title, j.HiringOrg.Name, j.Description, j.URL, j.DatePosted, i*1000+idx))
				}
			}
			continue
		}
		var graph ldGraph
		if err := json.Unmarshal([]byte(body), &graph); err == nil {
			for idx, j := range graph.Graph {
				if strings.EqualFold(j.Type, "JobPosting") {
					jobs = append(jobs, toPost(j.Title, j.HiringOrg.Name, j.Description, j.URL, j.DatePosted, i*1000+idx))
				}
			}
		}
	}
	return jobs
}

func toPost(title, company, description, jobURL, datePosted string, seed int) model.JobPost {
	title = strings.TrimSpace(title)
	company = strings.TrimSpace(company)
	post := model.JobPost{
		ID:          fmt.Sprintf("gd-%s-%s-%d", util.NormalizeSlug(title), util.NormalizeSlug(company), seed),
		Title:       title,
		CompanyName: company,
		Description: strings.TrimSpace(description),
		JobURL:      strings.TrimSpace(jobURL),
	}
	if post.JobURL == "" {
		post.JobURL = defaultURL
	}
	post.DatePosted = util.ParseDatePosted(datePosted)
	return post
}

func (s *Scraper) scrapeGraphQL(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	payload := buildGraphQLPayload(input)
	b, _ := json.Marshal([]any{payload})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.graphURL, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	for k, v := range glassdoorHeaders(s.csrfToken) {
		req.Header.Set(k, v)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("glassdoor graph status %d", resp.StatusCode)
	}
	body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return nil, err
	}
	return parseGraphQLJobs(body), nil
}

func glassdoorHeaders(csrfToken string) map[string]string {
	if strings.TrimSpace(csrfToken) == "" {
		csrfToken = fallbackCSRF
	}
	return map[string]string{
		"authority":                    "www.glassdoor.com",
		"accept":                       "*/*",
		"accept-language":              "en-US,en;q=0.9",
		"apollographql-client-name":    "job-search-next",
		"apollographql-client-version": "4.65.5",
		"content-type":                 "application/json",
		"gd-csrf-token":                csrfToken,
		"origin":                       "https://www.glassdoor.com",
		"referer":                      "https://www.glassdoor.com/",
		"sec-fetch-dest":               "empty",
		"sec-fetch-mode":               "cors",
		"sec-fetch-site":               "same-origin",
		"user-agent":                   "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/138.0.0.0 Safari/537.36",
	}
}

func (s *Scraper) bootstrapSession(ctx context.Context) error {
	seedURL := "https://www.glassdoor.com/Job/computer-science-jobs.htm"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, seedURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("accept-language", "en-US,en;q=0.9")
	req.Header.Set("user-agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/138.0.0.0 Safari/537.36")
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if m := reToken.FindStringSubmatch(string(body)); len(m) > 1 {
		s.csrfToken = m[1]
	}
	if strings.TrimSpace(s.csrfToken) == "" {
		s.csrfToken = fallbackCSRF
	}
	return nil
}

func buildGraphQLPayload(input model.ScraperInput) map[string]any {
	keyword := strings.TrimSpace(input.SearchTerm)
	if keyword == "" {
		keyword = "software engineer"
	}
	locationID := defaultLocationID
	locationType := "STATE"
	if strings.TrimSpace(input.Location) != "" && !strings.EqualFold(strings.TrimSpace(input.Location), "remote") {
		locationType = "CITY"
	}
	filterParams := make([]map[string]string, 0, 2)
	if input.EasyApply {
		filterParams = append(filterParams, map[string]string{"filterKey": "applicationType", "values": "1"})
	}
	if input.HoursOld > 0 {
		fromAgeDays := input.HoursOld / 24
		if fromAgeDays < 1 {
			fromAgeDays = 1
		}
		filterParams = append(filterParams, map[string]string{"filterKey": "fromAge", "values": strconv.Itoa(fromAgeDays)})
	}
	parameterURL := "IL.0,12_ISTATE" + strconv.Itoa(locationID)
	if locationType == "CITY" {
		parameterURL = "IL.0,12_ICITY" + strconv.Itoa(locationID)
	}
	return map[string]any{
		"operationName": "JobSearchResultsQuery",
		"variables": map[string]any{
			"excludeJobListingIds": []any{},
			"filterParams":         filterParams,
			"keyword":              keyword,
			"numJobsToShow":        30,
			"locationType":         locationType,
			"locationId":           locationID,
			"parameterUrlInput":    parameterURL,
			"pageNumber":           1,
			"pageCursor":           nil,
			"sort":                 "date",
		},
		"query": `query JobSearchResultsQuery($excludeJobListingIds: [Long!], $keyword: String, $locationId: Int, $locationType: LocationTypeEnum, $numJobsToShow: Int!, $pageCursor: String, $pageNumber: Int, $filterParams: [FilterParams], $parameterUrlInput: String) { jobListings(contextHolder: { searchParams: { excludeJobListingIds: $excludeJobListingIds, keyword: $keyword, locationId: $locationId, locationType: $locationType, numPerPage: $numJobsToShow, pageCursor: $pageCursor, pageNumber: $pageNumber, filterParams: $filterParams, parameterUrlInput: $parameterUrlInput, searchType: SR } }) { jobListings { jobview { header { employerNameFromSearch locationName locationType ageInDays } job { listingId jobTitleText description } } } } }`,
	}
}

func parseGraphQLJobs(raw []byte) []model.JobPost {
	var parsed []struct {
		Data struct {
			JobListings struct {
				JobListings []struct {
					JobView struct {
						Header struct {
							EmployerNameFromSearch string `json:"employerNameFromSearch"`
							LocationName           string `json:"locationName"`
							LocationType           string `json:"locationType"`
							AgeInDays              *int   `json:"ageInDays"`
						} `json:"header"`
						Job struct {
							ListingID    string `json:"listingId"`
							JobTitleText string `json:"jobTitleText"`
							Description  string `json:"description"`
						} `json:"job"`
					} `json:"jobview"`
				} `json:"jobListings"`
			} `json:"jobListings"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil || len(parsed) == 0 {
		return nil
	}
	out := make([]model.JobPost, 0, len(parsed[0].Data.JobListings.JobListings))
	for i, row := range parsed[0].Data.JobListings.JobListings {
		id := strings.TrimSpace(row.JobView.Job.ListingID)
		if id == "" {
			continue
		}
		title := strings.TrimSpace(row.JobView.Job.JobTitleText)
		company := strings.TrimSpace(row.JobView.Header.EmployerNameFromSearch)
		jobURL := "https://www.glassdoor.com/job-listing/j?jl=" + id
		jp := model.JobPost{ID: "gd-" + id, Title: title, CompanyName: company, JobURL: jobURL, Description: strings.TrimSpace(row.JobView.Job.Description)}
		if loc := parseGraphLocation(row.JobView.Header.LocationName); loc != nil {
			jp.Location = *loc
		}
		if row.JobView.Header.AgeInDays != nil {
			date := time.Now().AddDate(0, 0, -*row.JobView.Header.AgeInDays)
			jp.DatePosted = &date
		}
		if strings.EqualFold(strings.TrimSpace(row.JobView.Header.LocationType), "S") || strings.EqualFold(strings.TrimSpace(row.JobView.Header.LocationName), "Remote") {
			jp.IsRemote = true
		}
		if jp.Title == "" {
			jp.Title = fmt.Sprintf("Glassdoor Job %d", i+1)
		}
		out = append(out, jp)
	}
	return out
}

func parseGraphLocation(locationName string) *model.Location {
	locationName = strings.TrimSpace(locationName)
	if locationName == "" || strings.EqualFold(locationName, "Remote") {
		return nil
	}
	parts := strings.Split(locationName, ",")
	loc := &model.Location{}
	if len(parts) > 0 {
		loc.City = strings.TrimSpace(parts[0])
	}
	if len(parts) > 1 {
		loc.State = strings.TrimSpace(parts[1])
	}
	return loc
}
