package startupjobs_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/arinbalyan/scrappy/internal/model"
	startupjobs "github.com/arinbalyan/scrappy/internal/scraper/startupjobs"
)

func TestStartupJobsScrape(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"id":"1","title":"Founding Engineer","company_name":"StartCo","description":"Build product","remote":true,"location":"Remote","created_at":"2026-05-20T00:00:00Z","url":"https://startup.jobs/job/1"}]`))
	}))
	defer srv.Close()

	s := startupjobs.NewWithAPIURL(srv.Client(), srv.URL)
	jobs, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 5})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(jobs) != 1 || jobs[0].Title != "Founding Engineer" {
		t.Fatalf("unexpected jobs: %+v", jobs)
	}
}
