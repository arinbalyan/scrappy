package glassdoor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/util"
)

const defaultBaseURL = "https://www.glassdoor.com/"

// Fallback CSRF token used when extraction from homepage fails.
const defaultCSRFToken = "test-csrf-token"

var csrfRE = regexp.MustCompile(`gdCSRF\s*=\s*"([^"]+)"`)

// Browser-like headers matching the TypeScript constants.
var defaultHeaders = map[string]string{
	"authority":          "www.glassdoor.com",
	"accept":             "*/*",
	"accept-language":    "en-US,en;q=0.9",
	"content-type":       "application/json",
	"origin":             "https://www.glassdoor.com",
	"referer":            "https://www.glassdoor.com/",
	"sec-ch-ua":          `"Not_A Brand";v="99", "Google Chrome";v="120", "Chromium";v="120"`,
	"sec-ch-ua-mobile":   "?0",
	"sec-ch-ua-platform": `"macOS"`,
	"sec-fetch-dest":     "empty",
	"sec-fetch-mode":     "cors",
	"sec-fetch-site":     "same-origin",
	"user-agent":         "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
}

// jobSearchQuery is the GraphQL query for Glassdoor's JobSearchQuery operation.
// Copied from the TypeScript source (glassdoor.constants.ts).
const jobSearchQuery = `query JobSearchQuery(
  $keyword: String
  $locationId: Int
  $locationType: LocationTypeEnum
  $numPerPage: Int
  $pageCursor: String
  $filterParams: [FilterParamInput]
  $originalPageUrl: String
  $seoUrl: Boolean
) {
  jobListings(
    contextHolder: {
      searchParams: {
        keyword: $keyword
        locationId: $locationId
        locationType: $locationType
        numPerPage: $numPerPage
        pageCursor: $pageCursor
        filterParams: $filterParams
        originalPageUrl: $originalPageUrl
        seoUrl: $seoUrl
      }
    }
  ) {
    companyFilterOptions { id shortName }
    filterOptions { filterKey options { id label } }
    indeedCtk
    jobListingSeoLinks { linkItems { position url } }
    paginationCursors { cursor pageNumber }
    indexablePageCount
    searchResultsMetadata {
      searchCriteria { keyword impliedKeyword locationId locationType pageNumber seoFriendlyUrlInput }
      footerVO { countryMenu { childNavigationLinks { id link textKey } } }
      helpCenterDomain helpCenterLocale searchId
    }
    jobListings {
      jobview {
        header {
          adOrderId adOrderSponsorshipLevel ageInDays divisionEmployerName easyApply employer { id name shortName }
          employerNameFromSearch goc jobCountryId jobLink jobResultTrackingKey jobTitleId jobTitleText locId
          locationName locationType lowQualityApply payCurrency payPeriod payPeriodAdjustedPay { p10 p50 p90 }
          rating savedJobId seoJobLink sponsored normalizedJobTitle
        }
        job { descriptionFragments importConfigId jobTitleId jobTitleText listingId }
        overview { id name shortName squareLogoUrl }
      }
    }
  }
}`

// Scraper implements the scraper.Scraper interface for Glassdoor.
type Scraper struct {
	client  *http.Client
	baseURL string
}

// New creates a new Glassdoor scraper with the given HTTP client.
// If client is nil, a default one with retries and timeout is created.
func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Retries: 2, Timeout: 30 * time.Second})
	}
	return &Scraper{client: client, baseURL: defaultBaseURL}
}

// NewWithURLs creates a scraper with a custom base URL override.
func NewWithURLs(client *http.Client, baseURL string) *Scraper {
	s := New(client)
	if strings.TrimSpace(baseURL) != "" {
		s.baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/") + "/"
	}
	return s
}

// SiteName returns model.SiteGlassdoor.
func (s *Scraper) SiteName() model.Site { return model.SiteGlassdoor }

// Scrape performs the full Glassdoor job search flow:
//  1. Fetch homepage to extract CSRF token
//  2. POST GraphQL requests with cursor-based pagination
//  3. Parse job listings into model.JobPost records
//  4. Sleep ~5 seconds between pages to respect rate limits
func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	baseURL := s.baseURL
	if input.Country != "" {
		baseURL = glassdoorBaseURL(input.Country)
	}

	wanted := input.ResultsWanted
	if wanted <= 0 {
		wanted = 15
	}

	// Step 1: fetch CSRF token from the homepage.
	csrfToken := fetchCSRFToken(ctx, s.client, baseURL)
	util.Debug("glassdoor_csrf", map[string]any{"token_prefix": safePrefix(csrfToken, 8)})

	jobs := make([]model.JobPost, 0, wanted)
	seen := make(map[string]struct{})
	page := 1
	var paginationCursors []paginationCursorEntry

	for len(jobs) < wanted {
		select {
		case <-ctx.Done():
			return jobs, ctx.Err()
		default:
		}

		pageJobs, cursors, err := s.fetchPage(ctx, input, baseURL, csrfToken, page, paginationCursors)
		if err != nil {
			return nil, fmt.Errorf("glassdoor page %d: %w", page, err)
		}
		if len(pageJobs) == 0 {
			break
		}

		paginationCursors = cursors

		for _, j := range pageJobs {
			if j.ID == "" {
				continue
			}
			if _, ok := seen[j.ID]; ok {
				continue
			}
			seen[j.ID] = struct{}{}
			jobs = append(jobs, j)
			if len(jobs) >= wanted {
				break
			}
		}

		page++

		// Rate-limit: 5-second delay between pages (from TypeScript source).
		select {
		case <-ctx.Done():
			return jobs, ctx.Err()
		case <-time.After(5 * time.Second):
		}
	}

	if len(jobs) > wanted {
		jobs = jobs[:wanted]
	}

	return jobs, nil
}

// --- CSRF token ---

// fetchCSRFToken extracts the gdCSRF token from the Glassdoor homepage HTML.
// Returns the fallback token on any failure so the scraper can still attempt requests.
func fetchCSRFToken(ctx context.Context, client *http.Client, baseURL string) string {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL, nil)
	if err != nil {
		return defaultCSRFToken
	}
	setDefaultHeaders(req)

	resp, err := client.Do(req)
	if err != nil {
		return defaultCSRFToken
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return defaultCSRFToken
	}

	m := csrfRE.FindStringSubmatch(string(body))
	if len(m) < 2 || m[1] == "" {
		return defaultCSRFToken
	}
	return m[1]
}

// --- GraphQL page fetch ---

// fetchPage executes one GraphQL page request and returns parsed jobs plus
// updated pagination cursors.
func (s *Scraper) fetchPage(
	ctx context.Context,
	input model.ScraperInput,
	baseURL, csrfToken string,
	page int,
	cursors []paginationCursorEntry,
) ([]model.JobPost, []paginationCursorEntry, error) {
	vars := map[string]any{
		"keyword":    input.SearchTerm,
		"numPerPage": 30,
		"seoUrl":     false,
	}

	if page > 1 {
		if cursor := getCursorForPage(cursors, page); cursor != "" {
			vars["pageCursor"] = cursor
		}
	}

	var filterParams []map[string]string
	if input.IsRemote {
		filterParams = append(filterParams, map[string]string{
			"filterKey": "remoteWorkType",
			"values":    "1",
		})
	}
	if input.HoursOld > 0 {
		days := (input.HoursOld + 23) / 24 // ceiling division
		filterParams = append(filterParams, map[string]string{
			"filterKey": "fromAge",
			"values":    fmt.Sprintf("%d", days),
		})
	}
	if len(filterParams) > 0 {
		vars["filterParams"] = filterParams
	}

	gqlReq := gqlRequest{
		OperationName: "JobSearchQuery",
		Query:         jobSearchQuery,
		Variables:     vars,
	}

	body, err := json.Marshal(gqlReq)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"graph", bytes.NewReader(body))
	if err != nil {
		return nil, nil, fmt.Errorf("create request: %w", err)
	}
	setDefaultHeaders(req)
	req.Header.Set("gd-csrf-token", csrfToken)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, nil, fmt.Errorf("glassdoor api status %d", resp.StatusCode)
	}

	raw, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return nil, nil, fmt.Errorf("read body: %w", err)
	}

	var gqlResp gqlResponse
	if err := json.Unmarshal(raw, &gqlResp); err != nil {
		return nil, nil, fmt.Errorf("parse response: %w", err)
	}

	if gqlResp.Data == nil || gqlResp.Data.JobListings == nil {
		return nil, nil, nil
	}

	jl := gqlResp.Data.JobListings

	// Update pagination cursors from the response.
	outCursors := cursors
	if len(jl.PaginationCursors) > 0 {
		outCursors = jl.PaginationCursors
	}

	listings := jl.JobListings
	if len(listings) == 0 {
		return nil, outCursors, nil
	}

	jobs := make([]model.JobPost, 0, len(listings))
	for _, item := range listings {
		if item.JobView == nil {
			continue
		}
		jobs = append(jobs, toJobPost(item.JobView, baseURL))
	}

	return jobs, outCursors, nil
}

// --- Helpers ---

// glassdoorBaseURL maps a Country to the Glassdoor base URL.
func glassdoorBaseURL(c model.Country) string {
	switch c {
	case model.CountryUSA:
		return "https://www.glassdoor.com/"
	case model.CountryCanada:
		return "https://www.glassdoor.ca/"
	case model.CountryUK:
		return "https://www.glassdoor.co.uk/"
	case model.CountryGermany:
		return "https://www.glassdoor.de/"
	case model.CountryFrance:
		return "https://www.glassdoor.fr/"
	case model.CountryIndia:
		return "https://www.glassdoor.co.in/"
	case model.CountryAustralia:
		return "https://www.glassdoor.com.au/"
	default:
		// USA fallback for unknown countries.
		return "https://www.glassdoor.com/"
	}
}

// setDefaultHeaders applies browser-like headers to a request.
func setDefaultHeaders(req *http.Request) {
	for k, v := range defaultHeaders {
		// Only set if not already set (to allow overrides).
		if req.Header.Get(k) == "" {
			req.Header.Set(k, v)
		}
	}
}

// getCursorForPage finds the pagination cursor for a given page number.
func getCursorForPage(cursors []paginationCursorEntry, page int) string {
	for _, c := range cursors {
		if c.PageNumber == page {
			return c.Cursor
		}
	}
	return ""
}

// parseLocation splits a "City, State" location string into model.Location.
func parseLocation(locationName string) model.Location {
	if locationName == "" {
		return model.Location{}
	}
	parts := strings.SplitN(locationName, ", ", 2)
	loc := model.Location{City: parts[0]}
	if len(parts) > 1 {
		loc.State = parts[len(parts)-1]
	}
	return loc
}

// toJobPost converts a Glassdoor jobView into a model.JobPost.
func toJobPost(jv *jobView, baseURL string) model.JobPost {
	h := jv.Header
	if h == nil {
		return model.JobPost{}
	}
	j := jv.Job
	ov := jv.Overview

	// Job ID: "gd-" + adOrderId or listingId.
	jobID := ""
	if h.AdOrderID != nil && *h.AdOrderID > 0 {
		jobID = fmt.Sprintf("gd-%d", *h.AdOrderID)
	} else if j != nil && j.ListingID != nil && *j.ListingID > 0 {
		jobID = fmt.Sprintf("gd-%d", *j.ListingID)
	}

	// Title.
	title := h.JobTitleText
	if title == "" {
		title = "N/A"
	}

	// Company name: employerNameFromSearch → overview.shortName → employer.name.
	companyName := h.EmployerNameFromSearch
	if companyName == "" && ov != nil && ov.ShortName != "" {
		companyName = ov.ShortName
	}
	if companyName == "" && ov != nil && ov.Name != "" {
		companyName = ov.Name
	}
	if companyName == "" && h.Employer != nil && h.Employer.Name != "" {
		companyName = h.Employer.Name
	}

	// Job URL: seoJobLink → jobLink, ensure absolute.
	jobURL := ""
	if h.SeoJobLink != "" {
		jobURL = h.SeoJobLink
	} else if h.JobLink != "" {
		jobURL = h.JobLink
	}
	if jobURL != "" && !strings.HasPrefix(jobURL, "http") {
		jobURL = baseURL + strings.TrimLeft(jobURL, "/")
	}

	// Description: join fragments with newline.
	var description string
	if j != nil && len(j.DescriptionFragments) > 0 {
		description = strings.Join(j.DescriptionFragments, "\n")
	}

	// Compensation from payPeriodAdjustedPay (p10 = min, p90 = max).
	var comp *model.Compensation
	if h.PayPeriodAdjustedPay != nil {
		pay := h.PayPeriodAdjustedPay
		if pay.P10 != nil || pay.P50 != nil || pay.P90 != nil {
			interval := mapPayPeriod(h.PayPeriod)
			currency := h.PayCurrency
			if currency == "" {
				currency = "USD"
			}
			comp = &model.Compensation{
				Interval:  interval,
				MinAmount: pay.P10,
				MaxAmount: pay.P90,
				Currency:  currency,
			}
		}
	}

	// Location.
	location := parseLocation(h.LocationName)

	// Date posted: ageInDays → date calculated backward from now.
	var datePosted *time.Time
	if h.AgeInDays != nil {
		t := time.Now().AddDate(0, 0, -*h.AgeInDays)
		datePosted = &t
	}

	// Remote flag: locationType "S" means remote/single location type remote.
	isRemote := h.LocationType == "S"

	// Company logo.
	var companyLogo string
	if ov != nil {
		companyLogo = ov.SquareLogoURL
	}

	// Apply method.
	applyMethod := ""
	if h.EasyApply {
		applyMethod = "easy_apply"
	} else if jobURL != "" {
		applyMethod = "external_url"
	}

	return model.JobPost{
		ID:             jobID,
		Title:          title,
		CompanyName:    companyName,
		JobURL:         jobURL,
		Location:       location,
		Compensation:   comp,
		DatePosted:     datePosted,
		IsRemote:       isRemote,
		Description:    description,
		CompanyLogoURL: companyLogo,
		ApplyMethod:    applyMethod,
	}
}

// mapPayPeriod converts a Glassdoor pay period string to a CompensationInterval.
func mapPayPeriod(period string) model.CompensationInterval {
	switch strings.ToUpper(strings.TrimSpace(period)) {
	case "HOURLY", "HOUR":
		return model.IntervalHourly
	case "MONTHLY", "MONTH":
		return model.IntervalMonthly
	case "WEEKLY", "WEEK":
		return model.IntervalWeekly
	default:
		return model.IntervalYearly
	}
}

// safePrefix returns the first n characters of s, useful for logging secrets safely.
func safePrefix(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// --- JSON types ---

type gqlRequest struct {
	OperationName string `json:"operationName"`
	Query         string `json:"query"`
	Variables     any    `json:"variables"`
}

type gqlResponse struct {
	Data *gqlData `json:"data"`
}

type gqlData struct {
	JobListings *jobListingsData `json:"jobListings"`
}

type jobListingsData struct {
	PaginationCursors []paginationCursorEntry `json:"paginationCursors"`
	JobListings       []jobListingEntry       `json:"jobListings"`
}

type paginationCursorEntry struct {
	Cursor     string `json:"cursor"`
	PageNumber int    `json:"pageNumber"`
}

type jobListingEntry struct {
	JobView *jobView `json:"jobview"`
}

type jobView struct {
	Header   *jobHeader `json:"header"`
	Job      *jobBody   `json:"job"`
	Overview *overview  `json:"overview"`
}

type jobHeader struct {
	AdOrderID               *int             `json:"adOrderId"`
	AdOrderSponsorshipLevel *any             `json:"adOrderSponsorshipLevel"`
	AgeInDays               *int             `json:"ageInDays"`
	DivisionEmployerName    string           `json:"divisionEmployerName"`
	EasyApply               bool             `json:"easyApply"`
	Employer                *employerInfo    `json:"employer"`
	EmployerNameFromSearch  string           `json:"employerNameFromSearch"`
	Goc                     *any             `json:"goc"`
	JobCountryID            *int             `json:"jobCountryId"`
	JobLink                 string           `json:"jobLink"`
	JobResultTrackingKey    *any             `json:"jobResultTrackingKey"`
	JobTitleID              *int             `json:"jobTitleId"`
	JobTitleText            string           `json:"jobTitleText"`
	LocID                   *int             `json:"locId"`
	LocationName            string           `json:"locationName"`
	LocationType            string           `json:"locationType"`
	LowQualityApply         bool             `json:"lowQualityApply"`
	PayCurrency             string           `json:"payCurrency"`
	PayPeriod               string           `json:"payPeriod"`
	PayPeriodAdjustedPay    *payAdjustment   `json:"payPeriodAdjustedPay"`
	Rating                  *float64         `json:"rating"`
	SavedJobID              *any             `json:"savedJobId"`
	SeoJobLink              string           `json:"seoJobLink"`
	Sponsored               bool             `json:"sponsored"`
	NormalizedJobTitle      *any             `json:"normalizedJobTitle"`
}

type employerInfo struct {
	ID        *int    `json:"id"`
	Name      string  `json:"name"`
	ShortName string  `json:"shortName"`
}

type jobBody struct {
	DescriptionFragments []string `json:"descriptionFragments"`
	ImportConfigID       *any     `json:"importConfigId"`
	JobTitleID           *int     `json:"jobTitleId"`
	JobTitleText         string   `json:"jobTitleText"`
	ListingID            *int     `json:"listingId"`
}

type overview struct {
	ID           *int    `json:"id"`
	Name         string  `json:"name"`
	ShortName    string  `json:"shortName"`
	SquareLogoURL string  `json:"squareLogoUrl"`
}

type payAdjustment struct {
	P10 *float64 `json:"p10"`
	P50 *float64 `json:"p50"`
	P90 *float64 `json:"p90"`
}
