package wellfound_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/arinbalyan/scrappy/internal/model"
	wellfoundpkg "github.com/arinbalyan/scrappy/internal/scraper/wellfound"
)

func TestWellfoundScrape(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<a href="https://wellfound.com/jobs/1"><h2>Go Engineer</h2><span class="company">Acme</span></a>`))
	}))
	defer srv.Close()
	s := wellfoundpkg.NewWithListURL(srv.Client(), srv.URL)
	jobs, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 1})
	if err != nil || len(jobs) != 1 || jobs[0].CompanyName != "Acme" { t.Fatalf("unexpected wellfound result: %v %+v", err, jobs) }
}

func TestWellfoundScrapePrefersLDJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<script type="application/ld+json">{"@type":"JobPosting","title":"Senior Go Engineer","description":"Distributed systems","datePosted":"2026-05-20","employmentType":"FULL_TIME","url":"https://wellfound.com/jobs/42","hiringOrganization":{"name":"Acme"}}</script>`))
	}))
	defer srv.Close()
	s := wellfoundpkg.NewWithListURL(srv.Client(), srv.URL)
	jobs, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 1})
	if err != nil || len(jobs) != 1 || jobs[0].Title != "Senior Go Engineer" {
		t.Fatalf("unexpected wellfound ldjson result: %v %+v", err, jobs)
	}
	if jobs[0].DatePosted == nil || jobs[0].JobType != "full_time" {
		t.Fatalf("expected parsed ldjson fields: %+v", jobs[0])
	}
}
