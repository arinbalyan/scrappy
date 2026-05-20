package hiringcafe_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/arinbalyan/scrappy/internal/model"
	hiringcafe "github.com/arinbalyan/scrappy/internal/scraper/hiringcafe"
)

func TestHiringCafeScrape(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"ID":"1","Title":"Staff Engineer","Company":"Cafe","Location":"Remote","URL":"https://hc/job/1","Description":"Platform","PostedAt":"2026-05-20T00:00:00Z","Remote":true}]`))
	}))
	defer srv.Close()
	s := hiringcafe.NewWithAPIURL(srv.Client(), srv.URL)
	jobs, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 5})
	if err != nil || len(jobs) != 1 {
		t.Fatalf("err=%v jobs=%d", err, len(jobs))
	}
}
