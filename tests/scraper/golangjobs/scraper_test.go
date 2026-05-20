package golangjobs_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/arinbalyan/scrappy/internal/model"
	golangjobs "github.com/arinbalyan/scrappy/internal/scraper/golangjobs"
)

func TestGolangJobsScrape(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"ID":"1","Title":"Go Dev","Company":"Acme","Location":"Remote","URL":"https://go/job/1","Description":"Build","PostedAt":"2026-05-20T00:00:00Z"}]`))
	}))
	defer srv.Close()
	s := golangjobs.NewWithAPIURL(srv.Client(), srv.URL)
	jobs, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 5})
	if err != nil || len(jobs) != 1 {
		t.Fatalf("err=%v jobs=%d", err, len(jobs))
	}
}
