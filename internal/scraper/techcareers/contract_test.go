package techcareers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/arinbalyan/scrappy/internal/model"
)

const sampleHTML = `<!DOCTYPE html>
<html>
<body>
<article class="job-listing">
  <h2 class="job-title"><a href="/jobs/software-engineer-123">Software Engineer</a></h2>
  <div class="company">TechCorp</div>
  <div class="location">San Francisco, CA</div>
  <div class="date">2026-05-20</div>
  <div class="description">Build great software with Go and Python.</div>
</article>
<article class="job-card">
  <h2 class="job-title"><a href="https://www.techcareers.com/jobs/frontend-dev-456">Frontend Developer</a></h2>
  <div class="company">WebInc</div>
  <div class="location">Remote</div>
  <div class="posted-date">2026-05-21</div>
  <div class="snippet">React, TypeScript, CSS expertise needed.</div>
</article>
<article class="job-block">
  <h2 class="job-title"><a href="/jobs/empty-company-789">No Company</a></h2>
  <div class="location">New York, NY</div>
  <div class="date">2026-05-19</div>
  <div class="description">A job with no company name.</div>
</article>
</body>
</html>`

func TestScraper_SiteName(t *testing.T) {
	s := New(nil)
	if got := s.SiteName(); got != model.SiteTechCareers {
		t.Errorf("SiteName() = %q, want %q", got, model.SiteTechCareers)
	}
}

func TestScraper_Scrape(t *testing.T) {
	pageNum := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pageNum++
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		// Return jobs only on the first page, empty on subsequent pages
		if pageNum == 1 {
			w.Write([]byte(sampleHTML))
		} else {
			w.Write([]byte("<html><body></body></html>"))
		}
	}))
	defer ts.Close()

	s := NewWithBaseURL(nil, ts.URL)
	jobs, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 25})
	if err != nil {
		t.Fatalf("Scrape() returned error: %v", err)
	}

	if len(jobs) != 3 {
		t.Fatalf("expected 3 jobs, got %d", len(jobs))
	}

	// Job 0: Software Engineer at TechCorp
	j0 := jobs[0]
	if j0.Title != "Software Engineer" {
		t.Errorf("job[0].Title = %q, want %q", j0.Title, "Software Engineer")
	}
	if j0.CompanyName != "TechCorp" {
		t.Errorf("job[0].CompanyName = %q, want %q", j0.CompanyName, "TechCorp")
	}
	if j0.Site != string(model.SiteTechCareers) {
		t.Errorf("job[0].Site = %q, want %q", j0.Site, model.SiteTechCareers)
	}
	if j0.JobURL != "https://www.techcareers.com/jobs/software-engineer-123" {
		t.Errorf("job[0].JobURL = %q, want suffix /jobs/software-engineer-123", j0.JobURL)
	}
	if j0.Location.City != "San Francisco, CA" {
		t.Errorf("job[0].Location.City = %q, want %q", j0.Location.City, "San Francisco, CA")
	}
	if j0.DatePosted == nil {
		t.Error("job[0].DatePosted is nil, expected a parsed date")
	}
	if j0.Description != "Build great software with Go and Python." {
		t.Errorf("job[0].Description = %q", j0.Description)
	}

	// Job 1: Frontend Developer at WebInc
	j1 := jobs[1]
	if j1.Title != "Frontend Developer" {
		t.Errorf("job[1].Title = %q, want %q", j1.Title, "Frontend Developer")
	}
	if j1.CompanyName != "WebInc" {
		t.Errorf("job[1].CompanyName = %q, want %q", j1.CompanyName, "WebInc")
	}
	if j1.Location.City != "Remote" {
		t.Errorf("job[1].Location.City = %q, want %q", j1.Location.City, "Remote")
	}

	// Job 2: No Company (empty company field)
	j2 := jobs[2]
	if j2.Title != "No Company" {
		t.Errorf("job[2].Title = %q, want %q", j2.Title, "No Company")
	}
	if j2.CompanyName != "" {
		t.Errorf("job[2].CompanyName = %q, want empty", j2.CompanyName)
	}
}

func TestScraper_Scrape_HTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	s := NewWithBaseURL(nil, ts.URL)
	_, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 25})
	if err == nil {
		t.Fatal("expected error for 500 status, got nil")
	}
}

func TestScraper_Scrape_EmptyResponse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("<html><body>No jobs here</body></html>"))
	}))
	defer ts.Close()

	s := NewWithBaseURL(nil, ts.URL)
	_, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 25})
	if err == nil {
		t.Fatal("expected error for empty response, got nil")
	}
}

func TestScraper_Scrape_429(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer ts.Close()

	s := NewWithBaseURL(nil, ts.URL)
	_, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 25})
	if err == nil {
		t.Fatal("expected error for 429 status, got nil")
	}
}

func TestNewWithBaseURL_EmptyEndpoint(t *testing.T) {
	s1 := NewWithBaseURL(nil, "")
	s2 := New(nil)
	if s1.baseURL != s2.baseURL {
		t.Errorf("empty endpoint should not override base URL")
	}
}
