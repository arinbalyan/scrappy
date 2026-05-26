package successfactors

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

const sfPageSize = 20

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

func (s *Scraper) SiteName() model.Site { return model.SiteSuccessFactors }

type sfODataResponse struct {
	D *sfODataWrapper `json:"d"`
}

type sfODataWrapper struct {
	Results []sfJobPosting `json:"results"`
	Count   string         `json:"__count,omitempty"`
}

type sfJobPosting struct {
	JobReqID        string `json:"jobReqId,omitempty"`
	JobTitle        string `json:"jobTitle,omitempty"`
	FormattedTitle  string `json:"formattedJobTitle,omitempty"`
	JobDescription  string `json:"jobDescription,omitempty"`
	LocationObj     *struct {
		City    string `json:"city,omitempty"`
		State   string `json:"state,omitempty"`
		Country string `json:"country,omitempty"`
	} `json:"locationObj,omitempty"`
	Department      string `json:"department,omitempty"`
	PostingStartDate string `json:"postingStartDate,omitempty"`
	EmploymentType  string `json:"employmentType,omitempty"`
	CompanyName     string `json:"companyName,omitempty"`
	ExternalJobURL  string `json:"externalJobUrl,omitempty"`
}

func parseSfSlug(slug string) (instance, companyID string) {
	parts := strings.SplitN(slug, ":", 2)
	instance = parts[0]
	companyID = instance
	if len(parts) > 1 && parts[1] != "" {
		companyID = parts[1]
	}
	return
}

func buildODataURL(instance string) string {
	return fmt.Sprintf("https://%s.successfactors.com/odata/v2/JobRequisitionPosting", instance)
}

func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{"site": s.SiteName(), "results_wanted": input.ResultsWanted})

	seeds, src := ats.ResolveSeeds(input.SearchTerm, "SCRAPPY_SUCCESSFACTORS_SEEDS")
	if len(seeds) == 0 {
		return nil, fmt.Errorf("successfactors no seeds: set SCRAPPY_SUCCESSFACTORS_SEEDS or pass a company slug in --search (format: instance or instance:companyId, e.g. --search 'sap' or --search 'sap:SAP')")
	}
	util.Debug("successfactors_seeds", map[string]any{"seeds": seeds, "src": src})

	wanted := input.ResultsWanted
	if wanted <= 0 {
		wanted = 100
	}

	out := make([]model.JobPost, 0, wanted)
	for _, slug := range seeds {
		if len(out) >= wanted {
			break
		}

		instance, companyID := parseSfSlug(slug)
		u := s.buildURL(instance)
		if u == "" {
			u = buildODataURL(instance)
		}

		offset := 0
		for {
			if len(out) >= wanted {
				break
			}

			pageURL := fmt.Sprintf("%s?$select=jobReqId,jobTitle,jobDescription,locationObj,department,postingStartDate,employmentType,companyName,externalJobUrl&$top=%d&$skip=%d&$orderby=postingStartDate%%20desc&$format=json", u, sfPageSize, offset)
			resp := new(sfODataResponse)
			if err := ats.FetchJSON(ctx, s.client, pageURL, resp); err != nil {
				util.Warn("successfactors_fetch_fail", map[string]any{"instance": instance, "offset": offset, "err": err.Error()})
				break
			}

			if resp.D == nil || len(resp.D.Results) == 0 {
				break
			}

			for _, posting := range resp.D.Results {
				if len(out) >= wanted {
					break
				}
				jp := s.toJobPost(posting, instance, companyID)
				if jp != nil {
					out = append(out, *jp)
				}
			}

			offset += len(resp.D.Results)
			if len(resp.D.Results) < sfPageSize {
				break
			}
		}
	}

	if !util.HasMeaningfulJobs(out) {
		return nil, fmt.Errorf("successfactors no parseable jobs")
	}
	return out, nil
}

func (s *Scraper) buildURL(instance string) string {
	if s.apiURL != "" {
		return s.apiURL
	}
	return ""
}

func (s *Scraper) toJobPost(posting sfJobPosting, instance, companyID string) *model.JobPost {
	title := strings.TrimSpace(posting.JobTitle)
	if title == "" {
		title = strings.TrimSpace(posting.FormattedTitle)
	}
	if title == "" {
		return nil
	}

	loc := model.Location{}
	locStr := ""
	if posting.LocationObj != nil {
		loc.City = strings.TrimSpace(posting.LocationObj.City)
		loc.State = strings.TrimSpace(posting.LocationObj.State)
		loc.Country = strings.TrimSpace(posting.LocationObj.Country)
		locStr = loc.City
	}
	isRemote := strings.Contains(strings.ToLower(locStr), "remote")

	jobURL := strings.TrimSpace(posting.ExternalJobURL)
	if jobURL == "" {
		jobURL = fmt.Sprintf("https://%s.successfactors.com/career?company=%s&jobId=%s", instance, url.QueryEscape(companyID), url.QueryEscape(posting.JobReqID))
	}

	company := strings.TrimSpace(posting.CompanyName)
	if company == "" {
		company = companyID
	}

	jp := &model.JobPost{
		ID:          fmt.Sprintf("sf-%s-%s", instance, posting.JobReqID),
		Title:       title,
		CompanyName: company,
		JobURL:      jobURL,
		Location:    loc,
		IsRemote:    isRemote,
		Site:        string(model.SiteSuccessFactors),
		Department:  strings.TrimSpace(posting.Department),
		JobType:     strings.TrimSpace(posting.EmploymentType),
	}

	if posting.PostingStartDate != "" {
		jp.DatePosted = util.ParseDatePosted(posting.PostingStartDate)
	}

	return jp
}
