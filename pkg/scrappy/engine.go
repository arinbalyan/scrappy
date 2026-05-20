package scrappy

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/arinbalyan/scrappy/internal/dedup"
	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/quality"
	"github.com/arinbalyan/scrappy/internal/scraper"
	builtinscraper "github.com/arinbalyan/scrappy/internal/scraper/builtin"
	glassdoorscraper "github.com/arinbalyan/scrappy/internal/scraper/glassdoor"
	googlescraper "github.com/arinbalyan/scrappy/internal/scraper/google"
	indeedscraper "github.com/arinbalyan/scrappy/internal/scraper/indeed"
	linkedinscraper "github.com/arinbalyan/scrappy/internal/scraper/linkedin"
	remoteokscraper "github.com/arinbalyan/scrappy/internal/scraper/remoteok"
	remotivescraper "github.com/arinbalyan/scrappy/internal/scraper/remotive"
	wellfoundscraper "github.com/arinbalyan/scrappy/internal/scraper/wellfound"
	ziprecruiterscraper "github.com/arinbalyan/scrappy/internal/scraper/ziprecruiter"
)

type PostProcessor func(context.Context, *model.JobPost) error

type Engine struct {
	scrapers     map[model.Site]scraper.Scraper
	hooks        []PostProcessor
	telemetry    RunTelemetry
	siteFailOpen bool
}

func NewEngine() *Engine {
	s := []scraper.Scraper{
		indeedscraper.New(nil),
		linkedinscraper.New(nil),
		glassdoorscraper.New(nil),
		ziprecruiterscraper.New(nil),
		googlescraper.New(nil),
		wellfoundscraper.New(nil),
		remoteokscraper.New(nil),
		remotivescraper.New(nil),
		builtinscraper.New(nil),
	}
	m := make(map[model.Site]scraper.Scraper, len(s))
	for _, sc := range s {
		m[sc.SiteName()] = sc
	}
	return &Engine{scrapers: m, siteFailOpen: true}
}

func (e *Engine) SetSiteFailOpen(enabled bool) {
	e.siteFailOpen = enabled
}

func (e *Engine) Telemetry() RunTelemetry {
	return e.telemetry
}

func (e *Engine) RegisterHook(h PostProcessor) {
	e.hooks = append(e.hooks, h)
}

func (e *Engine) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	sites := input.Sites
	if len(sites) == 0 {
		sites = model.AllSites()
	}
	e.telemetry = RunTelemetry{Sites: make([]SiteTelemetry, 0, len(sites)), SuggestedSiteRPS: map[model.Site]int{}}

	all := make([]model.JobPost, 0)
	for _, site := range sites {
		sc, ok := e.scrapers[site]
		if !ok {
			continue
		}
		st := SiteTelemetry{Site: site, Attempted: true, StatusCodeCount: map[int]int{}}
		jobs, err := sc.Scrape(ctx, input)
		if err != nil {
			st.Error = err.Error()
			st.Success = false
			e.telemetry.SuggestedSiteRPS[site] = suggestRPS(input.SiteRPS[site], err)
			e.telemetry.Sites = append(e.telemetry.Sites, st)
			if e.siteFailOpen {
				continue
			}
			return nil, fmt.Errorf("scrape %s: %w", site, err)
		}
		st.Success = true
		st.ResultCount = len(jobs)
		if len(jobs) == 0 {
			st.EmptyPageRate = 1
		}
		e.telemetry.SuggestedSiteRPS[site] = suggestRPS(input.SiteRPS[site], nil)
		e.telemetry.Sites = append(e.telemetry.Sites, st)
		for i := range jobs {
			for _, h := range e.hooks {
				if err := h(ctx, &jobs[i]); err != nil {
					if e.siteFailOpen {
						continue
					}
					return nil, fmt.Errorf("post-process %s: %w", jobs[i].ID, err)
				}
			}
			jobs[i].QualityScore = quality.Score(&jobs[i])
		}
		all = append(all, jobs...)
	}

	all = dedup.DedupFilters(all, !input.Dedup, input.DedupByCompany, true)
	if input.MinScore > 0 {
		filtered := all[:0]
		for _, j := range all {
			if j.QualityScore >= input.MinScore {
				filtered = append(filtered, j)
			}
		}
		all = filtered
	}
	if input.EmailsOnly {
		filtered := all[:0]
		for _, j := range all {
			if len(j.Emails) > 0 {
				filtered = append(filtered, j)
			}
		}
		all = filtered
	}

	sort.SliceStable(all, func(i, j int) bool { return all[i].ID < all[j].ID })
	if input.ResultsWanted > 0 && len(all) > input.ResultsWanted {
		all = all[:input.ResultsWanted]
	}
	return all, nil
}

func suggestRPS(current int, err error) int {
	if current <= 0 {
		current = 3
	}
	if err == nil {
		if current < 10 {
			return current + 1
		}
		return current
	}
	e := err.Error()
	if containsAny(e, "429", "rate", "too many requests", "captcha") {
		if current > 1 {
			return current - 1
		}
		return 1
	}
	return current
}

func containsAny(s string, vals ...string) bool {
	s = strings.ToLower(s)
	for _, v := range vals {
		if strings.Contains(s, strings.ToLower(v)) {
			return true
		}
	}
	return false
}
