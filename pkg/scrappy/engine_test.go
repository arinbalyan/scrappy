package scrappy

import (
	"context"
	"testing"

	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/scraper"
)

type fakeScraper struct {
	site  model.Site
	calls []string
	jobs  map[string][]model.JobPost
}

func (f *fakeScraper) SiteName() model.Site { return f.site }

func (f *fakeScraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	f.calls = append(f.calls, input.SearchTerm)
	return f.jobs[input.SearchTerm], nil
}

func TestEngineAggregatesPerSiteMultipleSearchTerms(t *testing.T) {
	s := &fakeScraper{
		site: model.SiteIndeed,
		jobs: map[string][]model.JobPost{
			"golang": {
				{ID: "1", Title: "Go Dev", JobURL: "https://example.com/a", Description: "contact jobs@acme.com"},
			},
			"backend": {
				{ID: "2", Title: "Backend Dev", JobURL: "https://example.com/b", Description: "mail hr@acme.com"},
			},
		},
	}
	e := &Engine{scrapers: map[model.Site]scraper.Scraper{model.SiteIndeed: s}, siteFailOpen: true}
	input := model.ScraperInput{
		Sites:      []model.Site{model.SiteIndeed},
		SearchTerm: "default",
		SiteSearch: map[model.Site][]string{model.SiteIndeed: {"golang", "backend"}},
		Dedup:      true,
	}

	jobs, err := e.Scrape(context.Background(), input)
	if err != nil {
		t.Fatalf("scrape: %v", err)
	}
	if len(s.calls) != 2 || s.calls[0] != "golang" || s.calls[1] != "backend" {
		t.Fatalf("unexpected calls: %#v", s.calls)
	}
	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(jobs))
	}
	if len(jobs[0].Emails) == 0 && len(jobs[1].Emails) == 0 {
		t.Fatalf("expected extracted emails from descriptions")
	}
}

func TestEngineDedupsAcrossSameSiteSearchTermsByJobURL(t *testing.T) {
	s := &fakeScraper{
		site: model.SiteIndeed,
		jobs: map[string][]model.JobPost{
			"one": {{ID: "1", Title: "Role", JobURL: "https://example.com/a"}},
			"two": {{ID: "2", Title: "Role", JobURL: "https://example.com/a"}},
		},
	}
	e := &Engine{scrapers: map[model.Site]scraper.Scraper{model.SiteIndeed: s}, siteFailOpen: true}
	input := model.ScraperInput{
		Sites:      []model.Site{model.SiteIndeed},
		SiteSearch: map[model.Site][]string{model.SiteIndeed: {"one", "two"}},
		Dedup:      true,
	}

	jobs, err := e.Scrape(context.Background(), input)
	if err != nil {
		t.Fatalf("scrape: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 deduped job, got %d", len(jobs))
	}
}

func TestEngineEmailsOnlyFiltersAndExtractsFromDescriptionAndCompanyDescription(t *testing.T) {
	s := &fakeScraper{
		site: model.SiteIndeed,
		jobs: map[string][]model.JobPost{
			"golang": {
				{ID: "1", Title: "No email", JobURL: "https://example.com/no-email", Description: "plain text without contacts"},
				{ID: "2", Title: "Desc email", JobURL: "https://example.com/desc-email", Description: "contact eng@acme.com"},
				{ID: "3", Title: "Company desc email", JobURL: "https://example.com/company-email", CompanyDescription: "reach us at jobs@beta.com"},
			},
		},
	}
	e := &Engine{scrapers: map[model.Site]scraper.Scraper{model.SiteIndeed: s}, siteFailOpen: true}
	input := model.ScraperInput{
		Sites:      []model.Site{model.SiteIndeed},
		SearchTerm: "golang",
		EmailsOnly: true,
		Dedup:      true,
	}

	jobs, err := e.Scrape(context.Background(), input)
	if err != nil {
		t.Fatalf("scrape: %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("expected only 2 jobs with emails, got %d", len(jobs))
	}
	for _, j := range jobs {
		if len(j.Emails) == 0 {
			t.Fatalf("emails-only result contains job without emails: %s", j.ID)
		}
	}
}
