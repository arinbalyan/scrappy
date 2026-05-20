package adzuna_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/arinbalyan/scrappy/internal/model"
	adzuna "github.com/arinbalyan/scrappy/internal/scraper/adzuna"
)

func TestAdzunaScrape(t *testing.T) {
	t.Setenv("SCRAPPY_ADZUNA_APP_ID", "id")
	t.Setenv("SCRAPPY_ADZUNA_APP_KEY", "key")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"results":[{"id":"1","title":"Software Engineer","description":"Backend role","redirect_url":"https://example.com/job/1","created":"2026-05-20T00:00:00Z","company":{"display_name":"Acme"},"location":{"display_name":"Remote"}}]}`))
	}))
	defer srv.Close()

	s := adzuna.NewWithAPIURL(srv.Client(), srv.URL)
	jobs, err := s.Scrape(context.Background(), model.ScraperInput{SearchTerm: "software engineer", ResultsWanted: 10})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(jobs) != 1 || jobs[0].CompanyName != "Acme" {
		t.Fatalf("unexpected jobs: %+v", jobs)
	}
}
