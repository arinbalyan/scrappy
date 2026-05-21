package internshala_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	sut "github.com/arinbalyan/scrappy/internal/scraper/internshala"
)

func TestInternshalaParsesListings(t *testing.T) {
	html := `<div class="individual_job"><a class="job-title-href" href="/jobs/backend-developer-job-123">Backend Developer</a><div class="company-name">Acme</div><div class="location_link">Remote</div></div>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(html))
	}))
	defer srv.Close()
	s := sut.NewWithURLs(srv.Client(), srv.URL+"/jobs", srv.URL+"/internships")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	jobs, err := s.Scrape(ctx, model.ScraperInput{SearchTerm: "backend", ResultsWanted: 1})
	if err != nil || len(jobs) != 1 {
		t.Fatalf("expected 1 job and nil error, got jobs=%d err=%v", len(jobs), err)
	}
}

func TestInternshalaFailsOn429And503(t *testing.T) {
	for _, status := range []int{http.StatusTooManyRequests, http.StatusServiceUnavailable} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(status) }))
		s := sut.NewWithURLs(srv.Client(), srv.URL, srv.URL)
		_, err := s.Scrape(context.Background(), model.ScraperInput{SearchTerm: "x", ResultsWanted: 1})
		srv.Close()
		if err == nil {
			t.Fatalf("expected error for status %d", status)
		}
	}
}
