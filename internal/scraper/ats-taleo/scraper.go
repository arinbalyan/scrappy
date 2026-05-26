package taleo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/scraper/ats"
	"github.com/arinbalyan/scrappy/internal/util"
)

const (
	taleoPageSize = 25
)

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

func (s *Scraper) SiteName() model.Site { return model.SiteTaleo }

type taleoSearchPayload struct {
	MultilineEnabled bool              `json:"multilineEnabled"`
	SortingSelection *taleoSorting     `json:"sortingSelection"`
	PageNo           int               `json:"pageNo"`
	PageSize         int               `json:"pageSize"`
	Keyword          string            `json:"keyword"`
	Location         string            `json:"location"`
}

type taleoSorting struct {
	SortBySelectionParam string `json:"sortBySelectionParam"`
	AscendingSortingOrder string `json:"ascendingSortingOrder"`
}

type taleoJobListItem struct {
	ContestNo       string `json:"contestNo,omitempty"`
	Title           string `json:"title,omitempty"`
	PrimaryLocation string `json:"primaryLocation,omitempty"`
	JobField        string `json:"jobField,omitempty"`
	PostingDate     string `json:"postingDate,omitempty"`
	OpeningDate     string `json:"openingDate,omitempty"`
	Organization    string `json:"organization,omitempty"`
}

type taleoSearchResponse struct {
	RequisitionList []taleoJobListItem `json:"requisitionList"`
	TotalCount      int                `json:"totalCount,omitempty"`
}

func parseTaleoSlug(slug string) (company, careerSection string) {
	parts := strings.SplitN(slug, ":", 2)
	company = parts[0]
	careerSection = "ExternalCareerSite"
	if len(parts) > 1 && parts[1] != "" {
		careerSection = parts[1]
	}
	return
}

func buildSearchURL(company string) string {
	return fmt.Sprintf("https://%s.taleo.net/careersection/rest/jobboard/searchjobs", company)
}

func (s *Scraper) buildURL(company string) string {
	if s.apiURL != "" {
		return s.apiURL
	}
	return buildSearchURL(company)
}

func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{"site": s.SiteName(), "results_wanted": input.ResultsWanted})

	seeds, src := ats.ResolveSeeds(input.SearchTerm, "SCRAPPY_TALEO_SEEDS")
	if len(seeds) == 0 {
		return nil, fmt.Errorf("taleo no seeds: set SCRAPPY_TALEO_SEEDS or pass a company slug in --search (format: company or company:careerSection, e.g. --search 'oracle' or --search 'oracle:oraclecareersection')")
	}
	util.Debug("taleo_seeds", map[string]any{"seeds": seeds, "src": src})

	wanted := input.ResultsWanted
	if wanted <= 0 {
		wanted = 100
	}

	out := make([]model.JobPost, 0, wanted)
	for _, slug := range seeds {
		if len(out) >= wanted {
			break
		}

		company, careerSection := parseTaleoSlug(slug)
		u := s.buildURL(company)
		pageNo := 1

		for {
			if len(out) >= wanted {
				break
			}

			payload := taleoSearchPayload{
				MultilineEnabled: false,
				SortingSelection: &taleoSorting{
					SortBySelectionParam: "postedDate",
					AscendingSortingOrder: "false",
				},
				PageNo:   pageNo,
				PageSize: taleoPageSize,
				Keyword:  input.SearchTerm,
				Location: input.Location,
			}

			body, err := json.Marshal(payload)
			if err != nil {
				util.Warn("taleo_marshal_fail", map[string]any{"company": company, "err": err.Error()})
				break
			}

			req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(body))
			if err != nil {
				util.Warn("taleo_request_err", map[string]any{"company": company, "err": err.Error()})
				break
			}
			req.Header.Set("Accept", "application/json")
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("User-Agent", "Mozilla/5.0")

			resp, err := s.client.Do(req)
			if err != nil {
				util.Warn("taleo_fetch_fail", map[string]any{"company": company, "page": pageNo, "err": err.Error()})
				break
			}

			respBody, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
			resp.Body.Close()
			if err != nil {
				util.Warn("taleo_read_fail", map[string]any{"company": company, "err": err.Error()})
				break
			}

			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				util.Warn("taleo_status", map[string]any{"company": company, "page": pageNo, "status": resp.StatusCode})
				break
			}

			var searchResp taleoSearchResponse
			if err := json.Unmarshal(respBody, &searchResp); err != nil {
				util.Warn("taleo_decode_fail", map[string]any{"company": company, "err": err.Error()})
				break
			}

			if len(searchResp.RequisitionList) == 0 {
				break
			}

			for _, listing := range searchResp.RequisitionList {
				if len(out) >= wanted {
					break
				}
				jp := s.toJobPost(listing, company, careerSection)
				if jp != nil {
					out = append(out, *jp)
				}
			}

			if len(searchResp.RequisitionList) < taleoPageSize {
				break
			}
			pageNo++
		}
	}

	if !util.HasMeaningfulJobs(out) {
		return nil, fmt.Errorf("taleo no parseable jobs")
	}
	return out, nil
}

func (s *Scraper) toJobPost(listing taleoJobListItem, company, careerSection string) *model.JobPost {
	title := strings.TrimSpace(listing.Title)
	if title == "" {
		return nil
	}

	loc := model.Location{}
	isRemote := false
	locStr := strings.TrimSpace(listing.PrimaryLocation)
	if locStr != "" {
		loc.City = locStr
		if strings.Contains(strings.ToLower(locStr), "remote") {
			isRemote = true
		}
	}

	jobURL := fmt.Sprintf("https://%s.taleo.net/careersection/%s/jobdetail.ftl", company, careerSection)
	if listing.ContestNo != "" {
		jobURL = fmt.Sprintf("%s?job=%s", jobURL, listing.ContestNo)
	}

	org := strings.TrimSpace(listing.Organization)
	if org == "" {
		org = company
	}

	jp := &model.JobPost{
		ID:          fmt.Sprintf("taleo-%s", listing.ContestNo),
		Title:       title,
		CompanyName: org,
		JobURL:      jobURL,
		Location:    loc,
		IsRemote:    isRemote,
		Site:        string(model.SiteTaleo),
		Department:  strings.TrimSpace(listing.JobField),
	}

	dateStr := strings.TrimSpace(listing.PostingDate)
	if dateStr == "" {
		dateStr = strings.TrimSpace(listing.OpeningDate)
	}
	if dateStr != "" {
		jp.DatePosted = util.ParseDatePosted(dateStr)
	}

	return jp
}
