package builtin_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/arinbalyan/scrappy/internal/model"
	builtinpkg "github.com/arinbalyan/scrappy/internal/scraper/builtin"
)

func TestBuiltInScrape(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<a href="https://builtin.com/job/1"><h3>Backend Engineer</h3><span class="company">BuiltCo</span></a>`))
	}))
	defer srv.Close()
	s := builtinpkg.NewWithListURL(srv.Client(), srv.URL)
	jobs, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 1})
	if err != nil || len(jobs) != 1 || jobs[0].Title != "Backend Engineer" { t.Fatalf("unexpected builtin result: %v %+v", err, jobs) }
}

func TestBuiltInScrapePrefersLDJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<script type="application/ld+json">{"@type":"JobPosting","title":"Staff Backend Engineer","description":"Distributed systems","datePosted":"2026-05-20T10:00:00Z","url":"https://builtin.com/job/42","hiringOrganization":{"name":"BuiltCo"}}</script>`))
	}))
	defer srv.Close()
	s := builtinpkg.NewWithListURL(srv.Client(), srv.URL)
	jobs, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 1})
	if err != nil || len(jobs) != 1 || jobs[0].Title != "Staff Backend Engineer" {
		t.Fatalf("unexpected builtin ldjson result: %v %+v", err, jobs)
	}
	if jobs[0].DatePosted == nil || jobs[0].CompanyName != "BuiltCo" {
		t.Fatalf("expected parsed ldjson fields: %+v", jobs[0])
	}
}
