package bdjobs_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	sut "github.com/arinbalyan/scrappy/internal/scraper/bdjobs"
)

func TestBDJobsFailsOnEmptyResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<!-- no jobs -->`))
	}))
	defer srv.Close()
	s := &sut.Scraper{Client: srv.Client(), ListURL: srv.URL, AuthURL: srv.URL}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := s.Scrape(ctx, model.ScraperInput{SearchTerm: "developer", ResultsWanted: 5})
	if err == nil {
		t.Fatalf("expected error for empty upstream response, got nil")
	}
}

func TestBDJobsFailsOn429And503(t *testing.T) {
	for _, status := range []int{http.StatusTooManyRequests, http.StatusServiceUnavailable} {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(status)
		}))
		s := &sut.Scraper{Client: srv.Client(), ListURL: srv.URL, AuthURL: srv.URL}
		_, err := s.Scrape(ctx, model.ScraperInput{SearchTerm: "developer", ResultsWanted: 5})
		srv.Close()
		if err == nil {
			t.Fatalf("expected error for status %d", status)
		}
	}
}

func TestBDJobsReturnsJobsOnValidHTML(t *testing.T) {
	page := `<!DOCTYPE html><html><body>
	<a href="https://bdjobs.com/jobdetails.asp?id=1032276&amp;ln=1"><strong>Senior Software Engineer</strong></a>
	<a href="https://bdjobs.com/jobdetails.asp?id=1032277&amp;ln=1"><strong>Backend Developer</strong></a>
	</body></html>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(page))
	}))
	defer srv.Close()
	s := &sut.Scraper{Client: srv.Client(), ListURL: srv.URL, AuthURL: srv.URL}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	jobs, err := s.Scrape(ctx, model.ScraperInput{SearchTerm: "developer", ResultsWanted: 5})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(jobs) == 0 {
		t.Fatal("expected at least 1 job from valid HTML")
	}
	if jobs[0].Title == "" {
		t.Fatalf("expected non-empty title, got empty")
	}
	fmt.Printf("[bdjobs test] scraped %d jobs\n", len(jobs))
}

func TestBDJobsDetectsChallengePage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<html><body><div>captcha required</div></body></html>`))
	}))
	defer srv.Close()
	s := &sut.Scraper{Client: srv.Client(), ListURL: srv.URL, AuthURL: srv.URL}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := s.Scrape(ctx, model.ScraperInput{SearchTerm: "developer", ResultsWanted: 5})
	if err == nil {
		t.Fatal("expected error on challenge/captcha page")
	}
}
