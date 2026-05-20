package workable_jobs_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/arinbalyan/scrappy/internal/model"
	workable "github.com/arinbalyan/scrappy/internal/scraper/workable_jobs"
)

func TestWorkableScrape_FilterAndFields(t *testing.T) {
	t.Setenv("SCRAPPY_WORKABLE_SEEDS", "")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"results":[{"id":"1","title":"Software Engineer","url":"https://apply.workable.com/acme/j/1","description":"backend role","location":{"location_str":"Remote, US"},"remote":true,"created_at":"2026-05-20T00:00:00Z","employment_type":"Full Time","department":"Engineering"},{"id":"2","title":"Finance Manager","url":"https://apply.workable.com/acme/j/2","description":"finance","location":{"location_str":"NY, US"},"remote":false}]}`))
	}))
	defer srv.Close()

	s := workable.NewWithBaseURL(srv.Client(), srv.URL)
	jobs, err := s.Scrape(context.Background(), model.ScraperInput{SearchTerm: "software engineer", ResultsWanted: 10, WorkableSeeds: []string{"acme"}})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 got %d", len(jobs))
	}
	if jobs[0].Department != "Engineering" || !jobs[0].IsRemote {
		t.Fatalf("expected enriched remote engineering job, got %+v", jobs[0])
	}
}
