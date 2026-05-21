package himalayas_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/arinbalyan/scrappy/internal/model"
	himalayas "github.com/arinbalyan/scrappy/internal/scraper/himalayas"
)

func TestHimalayasScrape(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"jobs":[{"guid":"abc123","title":"Senior AI Engineer","excerpt":"Build LLM systems","companyName":"Acme AI","applicationLink":"https://himalayas.app/jobs/abc123","pubDate":1770000000000,"employmentType":"Full Time","categories":["Engineering"],"locationRestrictions":[{"name":"Worldwide"}]},{"guid":"abc124","title":"AI Platform Engineer","excerpt":"Ship infra","companyName":"Acme AI","applicationLink":"https://himalayas.app/jobs/abc124","pubDate":"2026-01-02T00:00:00Z","employmentType":"Full Time","categories":["Engineering"],"locationRestrictions":"Remote"}]}`))
	}))
	defer srv.Close()

	s := himalayas.NewWithAPIURL(srv.Client(), srv.URL)
	jobs, err := s.Scrape(context.Background(), model.ScraperInput{SearchTerm: "ai", ResultsWanted: 5})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(jobs))
	}
	if jobs[0].Title != "Senior AI Engineer" || jobs[0].CompanyName != "Acme AI" {
		t.Fatalf("unexpected job: %+v", jobs[0])
	}
	if jobs[1].Location.City != "Remote" {
		t.Fatalf("expected string location restriction to parse, got: %+v", jobs[1].Location)
	}
}
