package wuzzuf_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/arinbalyan/scrappy/internal/model"
	wuzzuf "github.com/arinbalyan/scrappy/internal/scraper/wuzzuf"
)

func TestWuzzufScrape(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"results":[{"ID":"1","Title":"Backend Engineer","URL":"https://wz/job/1","Description":"Go","PostedAt":"2026-05-20T00:00:00Z","company":{"name":"WZCo"},"location":{"name":"Cairo"}}]}`))
	}))
	defer srv.Close()
	s := wuzzuf.NewWithAPIURL(srv.Client(), srv.URL)
	jobs, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 5})
	if err != nil || len(jobs) != 1 {
		t.Fatalf("err=%v jobs=%d", err, len(jobs))
	}
}
