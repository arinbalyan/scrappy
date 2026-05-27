package wuzzuf_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	sut "github.com/arinbalyan/scrappy/internal/scraper/wuzzuf"
)

func TestWuzzufScrapeParsesJobs(t *testing.T) {
	html := `
	<html><body>
		<a href="/jobs/p/abc123-software-engineer">Software Engineer</a>
		<a href="/jobs/p/def456-ai-engineer">AI Engineer</a>
	</body></html>`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(html))
	}))
	defer srv.Close()

	s := sut.NewWithBaseURL(srv.Client(), srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	jobs, err := s.Scrape(ctx, model.ScraperInput{SearchTerm: "software", ResultsWanted: 2})
	if err != nil {
		t.Fatalf("Scrape returned error: %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(jobs))
	}
	if jobs[0].JobURL == "" || jobs[0].ID == "" {
		t.Fatalf("expected non-empty job URL and ID")
	}
}

func TestWuzzufScrapeEmptyResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<html><body></body></html>"))
	}))
	defer srv.Close()

	s := sut.NewWithBaseURL(srv.Client(), srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := s.Scrape(ctx, model.ScraperInput{SearchTerm: "gibberish", ResultsWanted: 5})
	if err == nil {
		t.Fatal("expected error for empty HTML with no job links")
	}
}

func TestWuzzufScrapeHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	s := sut.NewWithBaseURL(srv.Client(), srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := s.Scrape(ctx, model.ScraperInput{SearchTerm: "software", ResultsWanted: 5})
	if err == nil {
		t.Fatal("expected error for 500 status")
	}
}

func TestWuzzufScrapeContextTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Sleep longer than the context deadline
		time.Sleep(500 * time.Millisecond)
		_, _ = w.Write([]byte("<html><body></body></html>"))
	}))
	defer srv.Close()

	s := sut.NewWithBaseURL(srv.Client(), srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := s.Scrape(ctx, model.ScraperInput{SearchTerm: "software", ResultsWanted: 5})
	if err == nil {
		t.Fatal("expected error for context timeout")
	}
}

func TestWuzzufScrapeZeroResults(t *testing.T) {
	html := `
	<html><body>
		<div>No results found for this search.</div>
	</body></html>`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(html))
	}))
	defer srv.Close()

	s := sut.NewWithBaseURL(srv.Client(), srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := s.Scrape(ctx, model.ScraperInput{SearchTerm: "nonexistent", ResultsWanted: 5})
	if err == nil {
		t.Fatal("expected error for zero results")
	}
}
