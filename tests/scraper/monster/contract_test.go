package monster_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	sut "github.com/arinbalyan/scrappy/internal/scraper/monster"
)

func TestMonsterHappyPath(t *testing.T) {
	html := `<!DOCTYPE html><html><head></head><body>
<div data-testid="svx-job-result">
  <a data-testid="jobTitle" href="/jobs/abc123">Senior Software Engineer</a>
  <div data-testid="company">Google</div>
  <div data-testid="jobLocation">Mountain View, CA</div>
  <div data-testid="svx_jobCard-salary">$150K - $200K</div>
  <time datetime="2026-05-20">2 days ago</time>
</div>
<div data-testid="svx-job-result">
  <a data-testid="jobTitle" href="/jobs/def456">AI Engineer</a>
  <div data-testid="company">OpenAI</div>
  <div data-testid="jobLocation">San Francisco, CA</div>
  <div data-testid="svx_jobCard-salary">$180K - $250K</div>
  <time datetime="2026-05-19">3 days ago</time>
</div>
<div data-testid="svx-job-result">
  <a data-testid="jobTitle" href="/jobs/ghi789">Jr. AI Engineer</a>
  <div data-testid="company">Acme Corp</div>
  <div data-testid="jobLocation">Remote, US</div>
  <time datetime="2026-05-18">4 days ago</time>
</div>
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
	if jobs[0].Location.City != "Mountain View" || jobs[0].Location.State != "CA" {
		t.Errorf("expected location 'Mountain View, CA', got %+v", jobs[0].Location)
	}
	if jobs[0].IsRemote {
		t.Errorf("expected non-remote for Google job")
	}
	// Third job should be remote
	if !jobs[2].IsRemote {
		t.Errorf("expected remote for Acme Corp job")
	}
}

func TestMonsterFailsOn429And503(t *testing.T) {
	for _, status := range []int{http.StatusTooManyRequests, http.StatusServiceUnavailable} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(status) }))
		s := sut.NewWithURLs(srv.Client(), srv.URL)
		_, err := s.Scrape(context.Background(), model.ScraperInput{SearchTerm: "x", ResultsWanted: 1})
		srv.Close()
		if err == nil {
			t.Fatalf("expected error for status %d", status)
		}
	}
}

func TestMonsterEmptyResponseReturnsError(t *testing.T) {
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

func TestMonsterContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate slow response
		time.Sleep(5 * time.Second)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<html><body></body></html>"))
	}))
	defer srv.Close()

	s := sut.NewWithURLs(srv.Client(), srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	// Give the context time to expire
	time.Sleep(50 * time.Millisecond)
	_, err := s.Scrape(ctx, model.ScraperInput{SearchTerm: "x", ResultsWanted: 1})
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestMonsterPartialJobCards(t *testing.T) {
	// Test that jobs with missing optional fields still parse
	html := `<!DOCTYPE html><html><head></head><body>
<div data-testid="svx-job-result">
  <a data-testid="jobTitle" href="/jobs/abc123">Software Engineer</a>
  <div data-testid="company">Startup Inc</div>
  <time datetime="2026-05-20">2 days ago</time>
</div>
<div data-testid="svx-job-result">
  <a data-testid="jobTitle" href="/jobs/def456">Full Stack Developer</a>
  <div data-testid="company">Tech Co</div>
  <div data-testid="jobLocation">New York, NY</div>
  <time datetime="2026-05-19">3 days ago</time>
</div>
</body></html>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(html))
	}))
	defer srv.Close()

	s := sut.NewWithURLs(srv.Client(), srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	jobs, err := s.Scrape(ctx, model.ScraperInput{SearchTerm: "developer OR engineer", ResultsWanted: 2})
	if err != nil || len(jobs) != 2 {
		t.Fatalf("expected 2 jobs and nil error, got jobs=%d err=%v", len(jobs), err)
	}
	// First job has no location, that's OK — should still have title + company
	if jobs[0].Title != "Software Engineer" {
		t.Errorf("expected title 'Software Engineer', got %q", jobs[0].Title)
	}
}
