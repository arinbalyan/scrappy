package weworkremotely_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/arinbalyan/scrappy/internal/model"
	sut "github.com/arinbalyan/scrappy/internal/scraper/weworkremotely"
)

func TestWeworkremotelyScrapeSanitizesBrokenAmpersand(t *testing.T) {
	xml := `<?xml version="1.0"?><rss><channel><item><title>Acme: Senior Engineer</title><link>https://weworkremotely.com/remote-jobs/acme-senior-engineer</link><description>Build APIs & distributed systems</description><pubDate>Mon, 01 Jan 2024 00:00:00 GMT</pubDate></item></channel></rss>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(xml))
	}))
	defer srv.Close()

	s := sut.NewWithFeedURL(srv.Client(), srv.URL)
	jobs, err := s.Scrape(context.Background(), model.ScraperInput{SearchTerm: "software engineer", ResultsWanted: 5})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
	if jobs[0].Title != "Senior Engineer" {
		t.Fatalf("unexpected title: %q", jobs[0].Title)
	}
}
