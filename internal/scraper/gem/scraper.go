package gem

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

const gemBaseURL = "https://jobs.gem.com"
const gemAPIEndpoint = gemBaseURL + "/api/public/graphql/batch"

const defaultResultsWanted = 100

// ---- GraphQL queries ----

const jobBoardThemeQuery = `
query JobBoardTheme($boardId: String!) {
  publicBrandingTheme(externalId: $boardId) {
    id
    theme
    __typename
  }
}
`

const jobBoardListQuery = `
query JobBoardList($boardId: String!) {
  oatsExternalJobPostings(boardId: $boardId) {
    jobPostings {
      id
      extId
      title
      locations {
        id
        name
        city
        isoCountry
        isRemote
        extId
        __typename
      }
      job {
        id
        department {
          id
          name
          extId
          __typename
        }
        locationType
        employmentType
        __typename
      }
      __typename
    }
    __typename
  }
  oatsExternalJobPostingsFilters(boardId: $boardId) {
    type
    displayName
    rawValue
    value
    count
    __typename
  }
  jobBoardExternal(vanityUrlPath: $boardId) {
    id
    teamDisplayName
    descriptionHtml
    pageTitle
    __typename
  }
}
`

// Scraper fetches jobs from Gem career portals via GraphQL batch API.
type Scraper struct {
	client *http.Client
}

// New creates a new Gem scraper.
func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Timeout: 20 * time.Second})
	}
	return &Scraper{client: client}
}

// NewWithAPIURL exists for API symmetry; Gem uses GraphQL, not a simple API URL.
func NewWithAPIURL(client *http.Client, _ string) *Scraper {
	return New(client)
}

func (s *Scraper) SiteName() model.Site { return model.SiteGem }

// --- GraphQL response types ---

type graphQLOperation struct {
	OperationName string         `json:"operationName"`
	Variables     map[string]any `json:"variables"`
	Query         string         `json:"query"`
}

type gemLocation struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	City      string `json:"city"`
	ISOCountry string `json:"isoCountry"`
	IsRemote  *bool  `json:"isRemote"`
	ExtID     string `json:"extId"`
}

type gemDepartment struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	ExtID string `json:"extId"`
}

type gemJobMeta struct {
	ID             string          `json:"id"`
	Department     *gemDepartment  `json:"department"`
	LocationType   string          `json:"locationType"`
	EmploymentType string          `json:"employmentType"`
}

type gemJobPosting struct {
	ID        string         `json:"id"`
	ExtID     string         `json:"extId"`
	Title     string         `json:"title"`
	Locations []gemLocation  `json:"locations"`
	Job       *gemJobMeta    `json:"job"`
}

type gemOatsExternal struct {
	JobPostings []gemJobPosting `json:"jobPostings"`
}

type gemJobBoardExternal struct {
	ID              string `json:"id"`
	TeamDisplayName string `json:"teamDisplayName"`
	DescriptionHTML string `json:"descriptionHtml"`
	PageTitle       string `json:"pageTitle"`
}

type gemJobBoardListData struct {
	OatsExternalJobPostings *gemOatsExternal    `json:"oatsExternalJobPostings"`
	JobBoardExternal        *gemJobBoardExternal `json:"jobBoardExternal"`
}

type gemBatchEnvelope struct {
	Data   *gemJobBoardListData `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

// Scrape fetches jobs from Gem.
func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{"site": s.SiteName(), "results_wanted": input.ResultsWanted})

	seeds, src := ats.ResolveSeeds(input.SearchTerm, "SCRAPPY_GEM_SEEDS")
	if len(seeds) == 0 {
		return nil, fmt.Errorf("gem no seeds: set SCRAPPY_GEM_SEEDS or pass a company slug in --search")
	}
	util.Debug("gem_seeds", map[string]any{"seeds": seeds, "src": src})

	wanted := input.ResultsWanted
	if wanted <= 0 {
		wanted = defaultResultsWanted
	}

	out := make([]model.JobPost, 0, wanted)
	seen := make(map[string]bool)

	for _, slug := range seeds {
		if len(out) >= wanted {
			break
		}

		payload := []graphQLOperation{
			{
				OperationName: "JobBoardTheme",
				Variables:     map[string]any{"boardId": slug},
				Query:         jobBoardThemeQuery,
			},
			{
				OperationName: "JobBoardList",
				Variables:     map[string]any{"boardId": slug},
				Query:         jobBoardListQuery,
			},
		}

		body, err := json.Marshal(payload)
		if err != nil {
			util.Warn("gem_marshal_err", map[string]any{"slug": slug, "err": err.Error()})
			continue
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, gemAPIEndpoint, bytes.NewReader(body))
		if err != nil {
			util.Warn("gem_request_err", map[string]any{"slug": slug, "err": err.Error()})
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "*/*")
		req.Header.Set("User-Agent", "Mozilla/5.0")
		req.Header.Set("Origin", gemBaseURL)
		req.Header.Set("Referer", gemBaseURL)
		req.Header.Set("batch", "true")

		resp, err := s.client.Do(req)
		if err != nil {
			util.Warn("gem_fetch_fail", map[string]any{"slug": slug, "err": err.Error()})
			continue
		}

		respBody, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
		resp.Body.Close()
		if err != nil {
			util.Warn("gem_read_fail", map[string]any{"slug": slug, "err": err.Error()})
			continue
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			util.Warn("gem_status", map[string]any{"slug": slug, "status": resp.StatusCode})
			continue
		}

		var envelopes []gemBatchEnvelope
		if err := json.Unmarshal(respBody, &envelopes); err != nil {
			util.Warn("gem_decode_fail", map[string]any{"slug": slug, "err": err.Error()})
			continue
		}

		listEnv := pickJobBoardList(envelopes)
		if listEnv == nil {
			util.Warn("gem_no_list_envelope", map[string]any{"slug": slug})
			continue
		}

		if listEnv.Data == nil || listEnv.Data.OatsExternalJobPostings == nil {
			util.Warn("gem_no_job_postings", map[string]any{"slug": slug})
			continue
		}

		companyName := slug
		if listEnv.Data.JobBoardExternal != nil && strings.TrimSpace(listEnv.Data.JobBoardExternal.TeamDisplayName) != "" {
			companyName = strings.TrimSpace(listEnv.Data.JobBoardExternal.TeamDisplayName)
		}

		for _, posting := range listEnv.Data.OatsExternalJobPostings.JobPostings {
			if len(out) >= wanted {
				break
			}

			title := strings.TrimSpace(posting.Title)
			if title == "" {
				continue
			}

			// Use extId if available, fall back to id
			jobID := posting.ExtID
			if jobID == "" {
				jobID = posting.ID
			}
			if jobID == "" {
				continue
			}

			id := ats.BuildID("gem", slug, jobID)
			if seen[id] {
				continue
			}
			seen[id] = true

			// Location
			l := model.Location{}
			isRemote := false
			if len(posting.Locations) > 0 {
				loc := posting.Locations[0]
				l.City = strings.TrimSpace(loc.Name)
				if l.City == "" {
					l.City = strings.TrimSpace(loc.City)
				}
				if loc.IsRemote != nil && *loc.IsRemote {
					isRemote = true
				}
			}
			if !isRemote && posting.Job != nil {
				locType := strings.ToLower(posting.Job.LocationType)
				isRemote = strings.Contains(locType, "remote")
			}

			// Department
			dept := ""
			if posting.Job != nil && posting.Job.Department != nil {
				dept = strings.TrimSpace(posting.Job.Department.Name)
			}

			// Job URL
			jobURL := fmt.Sprintf("%s/%s/jobs/%s", gemBaseURL, slug, jobID)

			jp := model.JobPost{
				ID:          id,
				Title:       title,
				CompanyName: companyName,
				JobURL:      jobURL,
				Location:    l,
				IsRemote:    isRemote,
				Site:        string(s.SiteName()),
				Department:  dept,
				JobType:     normalizeEmploymentType(posting.Job),
			}
			out = append(out, jp)
		}
	}

	if !util.HasMeaningfulJobs(out) {
		return nil, fmt.Errorf("gem no parseable jobs")
	}
	return out, nil
}

func pickJobBoardList(envelopes []gemBatchEnvelope) *gemBatchEnvelope {
	for i := range envelopes {
		if envelopes[i].Data != nil && envelopes[i].Data.OatsExternalJobPostings != nil {
			return &envelopes[i]
		}
	}
	return nil
}

func normalizeEmploymentType(job *gemJobMeta) string {
	if job == nil {
		return ""
	}
	switch strings.ToLower(strings.TrimSpace(job.EmploymentType)) {
	case "fulltime", "full-time", "full_time":
		return "fulltime"
	case "parttime", "part-time", "part_time":
		return "parttime"
	case "contract", "contractor":
		return "contract"
	case "internship", "intern":
		return "internship"
	}
	return job.EmploymentType
}
