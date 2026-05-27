package oracle

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/scraper/ats"
	"github.com/arinbalyan/scrappy/internal/util"
)

const (
	oracleDefaultSiteNumber   = "CX_45001"
	oracleRecordsPerPage      = 100
	oracleDefaultResultsWanted = 100
	oracleMaxPages            = 50
	oracleDefaultSortBy       = "POSTING_DATES_DESC"
	oracleFinderName          = "findReqs"
	oracleRestPath            = "/hcmRestApi/resources/latest/recruitingCEJobRequisitions"
	oracleDefaultExpand       = "requisitionList.workLocation,requisitionList.otherWorkLocations,requisitionList.secondaryLocations,flexFieldsFacet.values,requisitionList.requisitionFlexFields"
)

var oracleDefaultFacets = []string{"LOCATIONS", "WORK_LOCATIONS", "WORKPLACE_TYPES", "TITLES", "CATEGORIES", "ORGANIZATIONS", "POSTING_DATES", "FLEX_FIELDS"}

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

func (s *Scraper) SiteName() model.Site { return model.SiteOracle }

type oracleRequisition struct {
	ID                 string `json:"Id"`
	Title              string `json:"Title"`
	PrimaryLocation    string `json:"PrimaryLocation,omitempty"`
	PostedDate         string `json:"PostedDate,omitempty"`
	EmployerName       string `json:"EmployerName,omitempty"`
	ExternalURL        string `json:"ExternalUrl,omitempty"`
	ExternalURLSeo     string `json:"ExternalUrlSeo,omitempty"`
	RequisitionNumber  string `json:"RequisitionNumber,omitempty"`
}

type oracleRequisitionWrapper struct {
	RequisitionList []oracleRequisition `json:"requisitionList"`
}

type oracleJobsResponse struct {
	Items []oracleRequisitionWrapper `json:"items"`
}

func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{"site": s.SiteName(), "results_wanted": input.ResultsWanted})

	tenant, err := s.resolveTenant(input.SearchTerm)
	if err != nil {
		return nil, fmt.Errorf("oracle tenant resolution: %w", err)
	}
	if tenant == nil {
		return nil, fmt.Errorf("oracle no tenant: set SCRAPPY_ORACLE_SEEDS or pass company URL/slug in --search")
	}

	wanted := input.ResultsWanted
	if wanted <= 0 {
		wanted = oracleDefaultResultsWanted
	}

	siteNumber := oracleDefaultSiteNumber

	var collected []oracleRequisition
	for page := 0; page < oracleMaxPages; page++ {
		if len(collected) >= wanted {
			break
		}
		offset := page * oracleRecordsPerPage
		u := s.buildPageURL(tenant.baseURL, siteNumber, offset)
		resp := new(oracleJobsResponse)
		if err := ats.FetchJSON(ctx, s.client, u, resp); err != nil {
			util.Warn("oracle_page_fail", map[string]any{"domain": tenant.domain, "offset": offset, "err": err.Error()})
			break
		}
		reqs := extractRequisitions(resp)
		if len(reqs) == 0 {
			break
		}
		for _, r := range reqs {
			if len(collected) >= wanted {
				break
			}
			collected = append(collected, r)
		}
		if len(reqs) < oracleRecordsPerPage {
			break
		}
	}

	out := make([]model.JobPost, 0, len(collected))
	for _, r := range collected {
		jp := s.toJobPost(r, tenant)
		out = append(out, jp)
	}

	if !util.HasMeaningfulJobs(out) {
		return nil, fmt.Errorf("oracle no parseable jobs")
	}
	return out, nil
}

type oracleTenant struct {
	baseURL     string
	domain      string
	companyName string
}

func (s *Scraper) resolveTenant(searchTerm string) (*oracleTenant, error) {
	seeds, _ := ats.ResolveSeeds(searchTerm, "SCRAPPY_ORACLE_SEEDS")
	if len(seeds) == 0 {
		return nil, nil
	}

	slug := strings.TrimSpace(seeds[0])
	if slug == "" {
		return nil, nil
	}

	if strings.HasPrefix(slug, "http") {
		return tenantFromURL(slug)
	}

	composed := composeOracleURL(slug)
	if composed != "" {
		return tenantFromURL(composed)
	}
	return nil, nil
}

func composeOracleURL(slug string) string {
	lastDash := strings.LastIndex(slug, "-")
	if lastDash <= 0 || lastDash == len(slug)-1 {
		return ""
	}
	subdomain := slug[:lastDash]
	region := slug[lastDash+1:]
	return fmt.Sprintf("https://%s.fa.%s.oraclecloud.com", subdomain, region)
}

func tenantFromURL(rawURL string) (*oracleTenant, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	baseURL := fmt.Sprintf("%s://%s", u.Scheme, u.Host)
	domain := u.Host
	companyName := extractCompanyName(domain)
	return &oracleTenant{baseURL: baseURL, domain: domain, companyName: companyName}, nil
}

func extractCompanyName(domain string) string {
	head := strings.SplitN(domain, ".", 2)[0]
	if head == "" {
		return domain
	}
	head = strings.ReplaceAll(head, "-", " ")
	if len(head) > 0 {
		head = strings.ToUpper(head[:1]) + head[1:]
	}
	return head
}

func (s *Scraper) buildPageURL(baseURL, siteNumber string, offset int) string {
	if s.apiURL != "" {
		return s.apiURL
	}

	facetsStr := strings.Join(oracleDefaultFacets, ";")
	finderParams := []string{
		fmt.Sprintf("siteNumber=%s", siteNumber),
		fmt.Sprintf("facetsList=%s", facetsStr),
		fmt.Sprintf("limit=%d", oracleRecordsPerPage),
	}
	if offset > 0 {
		finderParams = append(finderParams, fmt.Sprintf("offset=%d", offset))
	}
	finderParams = append(finderParams, fmt.Sprintf("sortBy=%s", oracleDefaultSortBy))
	finderString := strings.Join(finderParams, ",")

	query := fmt.Sprintf("?onlyData=true&expand=%s&finder=%s;%s", oracleDefaultExpand, oracleFinderName, finderString)
	return baseURL + oracleRestPath + query
}

func extractRequisitions(resp *oracleJobsResponse) []oracleRequisition {
	if len(resp.Items) == 0 {
		return nil
	}
	return resp.Items[0].RequisitionList
}

func (s *Scraper) toJobPost(req oracleRequisition, tenant *oracleTenant) model.JobPost {
	loc := model.Location{}
	isRemote := false
	if req.PrimaryLocation != "" {
		loc.City = req.PrimaryLocation
		if strings.Contains(strings.ToLower(req.PrimaryLocation), "remote") {
			isRemote = true
		}
	}

	jobURL := s.buildJobURL(req, tenant)
	companyName := req.EmployerName
	if companyName == "" {
		companyName = tenant.companyName
	}

	jp := model.JobPost{
		ID:          "oracle-" + req.ID,
		Title:       req.Title,
		CompanyName: companyName,
		JobURL:      jobURL,
		Location:    loc,
		IsRemote:    isRemote,
		Site:        string(model.SiteOracle),
	}
	if req.PostedDate != "" {
		jp.DatePosted = util.ParseDatePosted(req.PostedDate)
	}
	return jp
}

func (s *Scraper) buildJobURL(req oracleRequisition, tenant *oracleTenant) string {
	if req.ExternalURL != "" {
		return req.ExternalURL
	}
	slug := req.ExternalURLSeo
	if slug == "" {
		slug = req.ID
	}
	return fmt.Sprintf("%s/careers/job/%s", tenant.baseURL, slug)
}
