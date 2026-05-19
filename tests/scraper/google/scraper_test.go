package google_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/arinbalyan/scrappy/internal/model"
	googlepkg "github.com/arinbalyan/scrappy/internal/scraper/google"
)

func TestGoogleScrape(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<div data-job-id="abc"><div class="BjJfJf PUpOsf">Go Dev</div><div class="Qk80Jf">Acme</div></div>`))
	}))
	defer srv.Close()
	s := googlepkg.NewWithSearchURL(srv.Client(), srv.URL)
	jobs, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 1, SearchTerm: "go"})
	if err != nil || len(jobs) != 1 || jobs[0].CompanyName != "Acme" { t.Fatalf("unexpected google result: %v %+v", err, jobs) }
}

func TestGoogleScrapeLDJSONPreferred(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<script type="application/ld+json">{"@type":"JobPosting","title":"Platform Engineer","description":"Build infra","datePosted":"2026-05-20","hiringOrganization":{"name":"ExampleCo"}}</script>`))
	}))
	defer srv.Close()
	s := googlepkg.NewWithSearchURL(srv.Client(), srv.URL)
	jobs, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 1, SearchTerm: "platform"})
	if err != nil || len(jobs) != 1 || jobs[0].Title != "Platform Engineer" {
		t.Fatalf("unexpected google ldjson result: %v %+v", err, jobs)
	}
	if jobs[0].DatePosted == nil {
		t.Fatalf("expected date posted parsed from ld+json")
	}
}

func TestGoogleScrapeLDJSONGraphAndUniqueIDs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<script type="application/ld+json">{"@graph":[{"@type":"JobPosting","title":"Software Engineer","description":"A","datePosted":"2026-05-20T12:00:00Z","hiringOrganization":{"name":"Acme"}},{"@type":"JobPosting","title":"Software Engineer","description":"B","datePosted":"2026-05-20T13:00:00Z","hiringOrganization":{"name":"Globex"}}]}</script>`))
	}))
	defer srv.Close()
	s := googlepkg.NewWithSearchURL(srv.Client(), srv.URL)
	jobs, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 2, SearchTerm: "software"})
	if err != nil || len(jobs) != 2 {
		t.Fatalf("unexpected google graph result: %v %+v", err, jobs)
	}
	if jobs[0].ID == jobs[1].ID {
		t.Fatalf("expected unique ids, got duplicates: %s", jobs[0].ID)
	}
}
