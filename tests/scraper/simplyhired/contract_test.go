package simplyhired_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	sut "github.com/arinbalyan/scrappy/internal/scraper/simplyhired"
)

func TestHappyPath(t *testing.T) {
	html := `<!DOCTYPE html><html><body>
<div data-testid="searchSerpJob">
  <a data-testid="searchSerpJobTitle" href="/job/123">Senior Software Engineer</a>
  <span data-testid="companyName">Google</span>
  <span data-testid="searchSerpJobLocation">Mountain View, CA</span>
  <span data-testid="searchSerpJobSalaryEst">$150,000 - $200,000</span>
  <p class="jobposting-snippet">Building the next generation of search.</p>
</div>
<div data-testid="searchSerpJob">
  <a data-testid="searchSerpJobTitle" href="/job/456">AI Engineer</a>
  <span data-testid="companyName">OpenAI</span>
  <span data-testid="searchSerpJobLocation">San Francisco, CA</span>
  <span data-testid="searchSerpJobSalaryEst">$180,000 - $250,000</span>
  <p class="jobposting-snippet">Working on cutting-edge AI models.</p>
</div>
<div data-testid="searchSerpJob">
  <a data-testid="searchSerpJobTitle" href="/job/789">Backend Developer</a>
  <span data-testid="companyName">Acme Corp</span>
  <span data-testid="searchSerpJobLocation">Austin, TX</span>
  <p>A great backend role working on distributed systems.</p>
</div>
</body></html>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(html))
	}))
	defer srv.Close()

	s := sut.NewWithURLs(srv.Client(), srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	jobs, err := s.Scrape(ctx, model.ScraperInput{SearchTerm: "engineer", ResultsWanted: 3})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(jobs) != 3 {
		t.Fatalf("expected 3 jobs, got %d", len(jobs))
	}
	if jobs[0].Title != "Senior Software Engineer" {
		t.Errorf("expected title 'Senior Software Engineer', got %q", jobs[0].Title)
	}
	if jobs[0].CompanyName != "Google" {
		t.Errorf("expected company 'Google', got %q", jobs[0].CompanyName)
	}
	if jobs[0].JobURL != "https://www.simplyhired.com/job/123" {
		t.Errorf("expected job URL with prefix, got %q", jobs[0].JobURL)
	}
}

func TestErrorHandling429(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	s := sut.NewWithURLs(srv.Client(), srv.URL)
	_, err := s.Scrape(context.Background(), model.ScraperInput{SearchTerm: "engineer", ResultsWanted: 1})
	if err == nil {
		t.Fatal("expected error for 429 status")
	}
}

func TestErrorHandling503(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	s := sut.NewWithURLs(srv.Client(), srv.URL)
	_, err := s.Scrape(context.Background(), model.ScraperInput{SearchTerm: "engineer", ResultsWanted: 1})
	if err == nil {
		t.Fatal("expected error for 503 status")
	}
}

func TestEmptyResponse(t *testing.T) {
	html := `<html><body></body></html>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(html))
	}))
	defer srv.Close()

	s := sut.NewWithURLs(srv.Client(), srv.URL)
	_, err := s.Scrape(context.Background(), model.ScraperInput{SearchTerm: "engineer", ResultsWanted: 1})
	if err == nil {
		t.Fatal("expected error for empty response")
	}
}

func TestContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate a slow response
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<html><body><div data-testid=\"searchSerpJob\">test</div></body></html>"))
	}))
	defer srv.Close()

	s := sut.NewWithURLs(srv.Client(), srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	// Wait long enough for the context to cancel
	time.Sleep(10 * time.Millisecond)

	_, err := s.Scrape(ctx, model.ScraperInput{SearchTerm: "engineer", ResultsWanted: 1})
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}
