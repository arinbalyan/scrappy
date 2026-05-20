package workingnomads_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/arinbalyan/scrappy/internal/model"
	workingnomads "github.com/arinbalyan/scrappy/internal/scraper/workingnomads"
)

func TestWorkingNomadsScrape(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"slug":"job-1","title":"Platform Engineer","company_name":"NomadCo","location":"Anywhere","description":"Remote role","pub_date":"2026-05-20T00:00:00Z","url":"https://workingnomads/job-1"}]`))
	}))
	defer srv.Close()

	s := workingnomads.NewWithAPIURL(srv.Client(), srv.URL)
	jobs, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 5})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(jobs) != 1 || jobs[0].CompanyName != "NomadCo" {
		t.Fatalf("unexpected jobs: %+v", jobs)
	}
}
