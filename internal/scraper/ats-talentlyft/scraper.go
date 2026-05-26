package talentlyft

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

const talentlyftAPIURL = "https://api.talentlyft.com/v2/public"

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

func (s *Scraper) SiteName() model.Site { return model.SiteTalentLyft }

type tlJob struct {
	ID          interface{} `json:"Id,omitempty"`
	Title       string      `json:"Title,omitempty"`
	Description string      `json:"Description,omitempty"`
	Department  string      `json:"Department,omitempty"`
	Location    string      `json:"Location,omitempty"`
	CreatedAt   string      `json:"CreatedAt,omitempty"`
	URL         string      `json:"Url,omitempty"`
}

type tlResponse struct {
	Results []tlJob `json:"Results"`
}

func (s *Scraper) buildURL(slug string, perPage int) string {
	if s.apiURL != "" {
		return s.apiURL
	}
	return fmt.Sprintf("%s/%s/jobs?page=1&perPage=%d", talentlyftAPIURL, url.PathEscape(slug), perPage)
}

func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{"site": s.SiteName(), "results_wanted": input.ResultsWanted})

	seeds, src := ats.ResolveSeeds(input.SearchTerm, "SCRAPPY_TALENTLYFT_SEEDS")
	if len(seeds) == 0 {
		return nil, fmt.Errorf("talentlyft no seeds: set SCRAPPY_TALENTLYFT_SEEDS or pass a company slug in --search")
	}
	util.Debug("talentlyft_seeds", map[string]any{"seeds": seeds, "src": src})

	wanted := input.ResultsWanted
	if wanted <= 0 {
		wanted = 20
	}

	out := make([]model.JobPost, 0, wanted)
	for _, slug := range seeds {
		if len(out) >= wanted {
			break
		}

		u := s.buildURL(slug, wanted)
		resp := new(tlResponse)
		if err := ats.FetchJSON(ctx, s.client, u, resp); err != nil {
			util.Warn("talentlyft_fetch_fail", map[string]any{"slug": slug, "err": err.Error()})
			continue
		}

		for _, job := range resp.Results {
			if len(out) >= wanted {
				break
			}
			title := strings.TrimSpace(job.Title)
			if title == "" {
				continue
			}

			loc := model.Location{}
			isRemote := false
			locStr := strings.TrimSpace(job.Location)
			if locStr != "" {
				loc.City = locStr
				if strings.Contains(strings.ToLower(locStr), "remote") {
					isRemote = true
				}
			}

			jobURL := strings.TrimSpace(job.URL)

			jp := model.JobPost{
				ID:          fmt.Sprintf("talentlyft-%v", job.ID),
				Title:       title,
				CompanyName: slug,
				JobURL:      jobURL,
				Location:    loc,
				IsRemote:    isRemote,
				Description: util.StripHTML(strings.TrimSpace(job.Description)),
				Site:        string(model.SiteTalentLyft),
				Department:  strings.TrimSpace(job.Department),
			}
			if job.CreatedAt != "" {
				jp.DatePosted = util.ParseDatePosted(job.CreatedAt)
			}
			out = append(out, jp)
		}
	}

	if !util.HasMeaningfulJobs(out) {
		return nil, fmt.Errorf("talentlyft no parseable jobs")
	}
	return out, nil
}
