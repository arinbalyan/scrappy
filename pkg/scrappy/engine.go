package scrappy

import (
	"context"
	"fmt"
	"sort"

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
	scrapers map[model.Site]scraper.Scraper
	hooks    []PostProcessor
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
	return &Engine{scrapers: m}
}

func (e *Engine) RegisterHook(h PostProcessor) {
	e.hooks = append(e.hooks, h)
}

func (e *Engine) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	sites := input.Sites
	if len(sites) == 0 {
		sites = model.AllSites()
	}

	all := make([]model.JobPost, 0)
	for _, site := range sites {
		sc, ok := e.scrapers[site]
		if !ok {
			continue
		}
		jobs, err := sc.Scrape(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("scrape %s: %w", site, err)
		}
		for i := range jobs {
			for _, h := range e.hooks {
				if err := h(ctx, &jobs[i]); err != nil {
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
