package seek_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/arinbalyan/scrappy/internal/model"
	seek "github.com/arinbalyan/scrappy/internal/scraper/seek"
)

func TestSeekScrape(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"id":"abc","title":"Go Engineer","teaser":"Build APIs","advertiser":{"description":"Acme"},"location":"Remote","listingDate":"2026-05-20T00:00:00Z","jobUrl":"https://seek/job/abc"}]}`))
	}))
	defer srv.Close()

	s := seek.NewWithAPIURL(srv.Client(), srv.URL)
	jobs, err := s.Scrape(context.Background(), model.ScraperInput{SearchTerm: "go", ResultsWanted: 5})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(jobs) != 1 || jobs[0].Title != "Go Engineer" {
		t.Fatalf("unexpected jobs: %+v", jobs)
	}
}
