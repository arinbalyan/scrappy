package avature

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/scraper/ats"
	"github.com/arinbalyan/scrappy/internal/util"
)

const defaultResultsWanted = 100
const recordsPerPage = 12
const maxPages = 50

var (
	jobLinkRe     = regexp.MustCompile(`(?i)(/careers/JobDetail/[^"'\s]+)`)
	titleInLinkRe = regexp.MustCompile(`(?i)title="([^"]+)"`)
	locationRe    = regexp.MustCompile(`(?i)(?:class|className)\s*=\s*"[^"]*(?:location|job-location)[^"]*"[^>]*>(.*?)<`)
)

// Scraper fetches jobs from Avature-powered career portals.
type Scraper struct {
	client *http.Client
}

// New creates a new Avature scraper.
func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Timeout: 20 * time.Second})
	}
	return &Scraper{client: client}
}

// NewWithAPIURL exists for API symmetry.
func NewWithAPIURL(client *http.Client, _ string) *Scraper {
	return New(client)
}

func (s *Scraper) SiteName() model.Site { return model.SiteAvature }

// Scrape fetches jobs from an Avature career portal.
func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{"site": s.SiteName(), "results_wanted": input.ResultsWanted})

	seeds, src := ats.ResolveSeeds(input.SearchTerm, "SCRAPPY_AVATURE_SEEDS")
	if len(seeds) == 0 {
		return nil, fmt.Errorf("avature no seeds: set SCRAPPY_AVATURE_SEEDS or pass a company slug in --search")
	}
	// If the search term was used as slug and it looks like a search phrase,
	// return early — Avature needs a company slug like 'colgate-palmolive', not a search string.
	if src == ats.SeedFromSearch && (strings.ContainsAny(seeds[0], " \"") || strings.Contains(seeds[0], "OR")) {
		return nil, fmt.Errorf("avature: no tenant slugs — got search term %q; set SCRAPPY_AVATURE_SEEDS or pass --search 'company-slug'", seeds[0])
	}
	util.Debug("avature_seeds", map[string]any{"seeds": seeds, "src": src})

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
		baseURL := fmt.Sprintf("https://%s.avature.net", slug)
		companyName := deriveCompanyName(slug)
		jobs, err := s.fetchAllPages(ctx, baseURL, companyName, wanted)
		if err != nil {
			util.Warn("avature_fetch_fail", map[string]any{"slug": slug, "err": err.Error()})
			continue
		}
		for _, j := range jobs {
			if seen[j.ID] {
				continue
			}
			seen[j.ID] = true
			out = append(out, j)
		}
	}

	if !util.HasMeaningfulJobs(out) {
		return nil, fmt.Errorf("avature no parseable jobs")
	}
	return out, nil
}

func deriveCompanyName(slug string) string {
	return strings.Title(strings.ReplaceAll(slug, "-", " "))
}

func (s *Scraper) fetchAllPages(ctx context.Context, baseURL, companyName string, wanted int) ([]model.JobPost, error) {
	var all []model.JobPost

	for page := 0; page < maxPages && len(all) < wanted; page++ {
		offset := page * recordsPerPage
		u := fmt.Sprintf("%s/careers/SearchJobs/?jobOffset=%d&jobRecordsPerPage=%d", baseURL, offset, recordsPerPage)

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "text/html,application/xhtml+xml")
		req.Header.Set("User-Agent", "Mozilla/5.0")

		resp, err := s.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("avature page: %w", err)
		}

		body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("avature read: %w", err)
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("avature status %d", resp.StatusCode)
		}

		jobs := parseListings(string(body), baseURL, companyName, len(all), wanted-len(all))
		if len(jobs) == 0 {
			break
		}
		all = append(all, jobs...)

		if len(jobs) < recordsPerPage {
			break
		}
	}
	return all, nil
}

func parseListings(html, baseURL, companyName string, startIdx, wanted int) []model.JobPost {
	linkMatches := jobLinkRe.FindAllStringSubmatch(html, -1)
	if len(linkMatches) == 0 {
		return nil
	}

	titleMatches := titleInLinkRe.FindAllStringSubmatch(html, -1)
	locMatches := locationRe.FindAllStringSubmatch(html, -1)

	var out []model.JobPost
	for i, m := range linkMatches {
		if len(out) >= wanted {
			break
		}
		href := strings.TrimSpace(m[1])
		if href == "" {
			continue
		}

		jobURL := baseURL + href
		id := ats.BuildID("avature", baseURL, href)

		title := ""
		if i < len(titleMatches) {
			title = strings.TrimSpace(titleMatches[i][1])
		}
		if title == "" {
			title = "No title"
		}

		l := model.Location{}
		isRemote := false
		if i < len(locMatches) {
			locStr := strings.TrimSpace(stripTags(locMatches[i][1]))
			if strings.Contains(strings.ToLower(locStr), "remote") {
				isRemote = true
			}
			l.City = locStr
		}

		jp := model.JobPost{
			ID:          id,
			Title:       title,
			CompanyName: companyName,
			JobURL:      jobURL,
			Location:    l,
			IsRemote:    isRemote,
			Site:        string(model.SiteAvature),
		}
		out = append(out, jp)
	}
	return out
}

func stripTags(s string) string {
	return regexp.MustCompile(`<[^>]*>`).ReplaceAllString(s, "")
}
