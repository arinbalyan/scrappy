package jobindex_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/arinbalyan/scrappy/internal/model"
	jobindex "github.com/arinbalyan/scrappy/internal/scraper/jobindex"
)

func TestJobindexScrape(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"ID":"1","Title":"Engineer","Company":"Nordic","Location":"Copenhagen","URL":"https://ji/job/1","Description":"Role","PostedAt":"2026-05-20T00:00:00Z"}]`))
	}))
	defer srv.Close()
	s := jobindex.NewWithAPIURL(srv.Client(), srv.URL)
	jobs, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 5})
	if err != nil || len(jobs) != 1 {
		t.Fatalf("err=%v jobs=%d", err, len(jobs))
	}
}
