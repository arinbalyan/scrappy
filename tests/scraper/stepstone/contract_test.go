package stepstone_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	sut "github.com/arinbalyan/scrappy/internal/scraper/stepstone"
)

func TestStepStoneHappyPath(t *testing.T) {
	html := `<!DOCTYPE html><html><head></head><body>
<article data-testid="job-item">
  <h2><a data-at="job-item-title" href="/stellenangebote--senior-software-engineer">Senior Software Engineer</a></h2>
  <div data-at="job-item-company-name">Google</div>
  <div data-at="job-item-location">Berlin, Germany</div>
</article>
<article data-testid="job-item">
  <h2><a data-at="job-item-title" href="/stellenangebote--ai-engineer">AI Engineer</a></h2>
  <div data-at="job-item-company-name">OpenAI</div>
  <div data-at="job-item-location">Munich, Germany</div>
</article>
<article data-testid="job-item">
  <h2><a data-at="job-item-title" href="/stellenangebote--backend-developer">Backend Developer</a></h2>
  <div data-at="job-item-company-name">Acme Corp</div>
  <div data-at="job-item-location">Hamburg, Germany</div>
</article>
</body></html>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(html))
	}))
	defer srv.Close()

	s := sut.NewWithURLs(srv.Client(), srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	jobs, err := s.Scrape(ctx, model.ScraperInput{SearchTerm: "engineer", ResultsWanted: 3})
	if err != nil || len(jobs) != 3 {
		t.Fatalf("expected 3 jobs and nil error, got jobs=%d err=%v", len(jobs), err)
	}
	// Verify fields for first job
	if jobs[0].Title != "Senior Software Engineer" {
		t.Errorf("expected title 'Senior Software Engineer', got %q", jobs[0].Title)
	}
	if jobs[0].CompanyName != "Google" {
		t.Errorf("expected company 'Google', got %q", jobs[0].CompanyName)
	}
	if jobs[0].Location.City != "Berlin" {
		t.Errorf("expected city 'Berlin', got %q", jobs[0].Location.City)
	}
}

func TestStepStoneFailsOn429And503(t *testing.T) {
	for _, status := range []int{http.StatusTooManyRequests, http.StatusServiceUnavailable} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(status)
		}))
		s := sut.NewWithURLs(srv.Client(), srv.URL)
		_, err := s.Scrape(context.Background(), model.ScraperInput{SearchTerm: "x", ResultsWanted: 1})
		srv.Close()
		if err == nil {
			t.Fatalf("expected error for status %d", status)
		}
	}
}

func TestStepStoneEmptyResponseReturnsError(t *testing.T) {
	html := `<html><body></body></html>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(html))
	}))
	defer srv.Close()

	s := sut.NewWithURLs(srv.Client(), srv.URL)
	_, err := s.Scrape(context.Background(), model.ScraperInput{SearchTerm: "x", ResultsWanted: 1})
	if err == nil {
		t.Fatal("expected error for empty response")
	}
}

func TestStepStoneContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate slow response.
		time.Sleep(5 * time.Second)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<html><body></body></html>"))
	}))
	defer srv.Close()

	s := sut.NewWithURLs(srv.Client(), srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	// Give context time to propagate.
	time.Sleep(50 * time.Millisecond)
	_, err := s.Scrape(ctx, model.ScraperInput{SearchTerm: "x", ResultsWanted: 1})
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}
