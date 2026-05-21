package remotive_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	sut "github.com/arinbalyan/scrappy/internal/scraper/remotive"
)

func TestRemotiveFailsOnEmptyResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"jobs":[]}`))
	}))
	defer srv.Close()
	s := sut.NewWithAPIURL(srv.Client(), srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	jobs, err := s.Scrape(ctx, model.ScraperInput{SearchTerm: "software engineer", ResultsWanted: 5})
	if err == nil && len(jobs) == 0 {
		t.Fatalf("expected non-empty jobs or explicit error for empty upstream response")
	}
}

func TestRemotiveFailsOn429And503(t *testing.T) {
	for _, status := range []int{http.StatusTooManyRequests, http.StatusServiceUnavailable} {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(status)
		}))
		s := sut.NewWithAPIURL(srv.Client(), srv.URL)
		_, err := s.Scrape(ctx, model.ScraperInput{SearchTerm: "software engineer", ResultsWanted: 5})
		srv.Close()
		if err == nil {
			t.Fatalf("expected error for status %d", status)
		}
	}
}
