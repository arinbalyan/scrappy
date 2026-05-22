package jooble_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	sut "github.com/arinbalyan/scrappy/internal/scraper/jooble"
)

// sampleJobHTML simulates a Jooble search results page with job listings.
// The HTML structure mirrors real Jooble pages: div cards with job-title links,
// company/location/detail spans, and salary/description/date elements.
const sampleJobHTML = `<!DOCTYPE html>
<html>
<head><title>Jobs - Jooble</title></head>
<body>
<div class="search-results">
  <div class="job-card" data-id="abc123">
    <a class="job-title" href="/software-engineer-sf">Senior Software Engineer</a>
    <div class="job-info">
      <span class="company">Google</span>
      <span class="location">San Francisco, CA</span>
      <span class="salary">$150,000 - $200,000</span>
      <div class="description">Build and maintain distributed systems.</div>
      <span class="date">Posted 2 days ago</span>
      <span class="job-type">Full-time</span>
    </div>
  </div>
  <div class="job-card" data-id="def456">
    <a class="job-title" href="/ml-engineer-nyc">Machine Learning Engineer</a>
    <div class="job-info">
      <span class="company">OpenAI</span>
      <span class="location">New York, NY</span>
      <span class="salary">$180,000 - $250,000</span>
      <div class="description">Design and train large language models.</div>
      <span class="date">Posted Today</span>
      <span class="job-type">Full-time</span>
    </div>
  </div>
  <div class="job-card" data-id="ghi789">
    <a class="job-title" href="/backend-dev-remote">Backend Developer</a>
    <div class="job-info">
      <span class="company">Acme Corp</span>
      <span class="location">Remote</span>
      <div class="description">Build APIs and microservices.</div>
      <span class="date">Posted 5 days ago</span>
    </div>
  </div>
</div>
</body>
</html>`

func TestJoobleParsesJobs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(sampleJobHTML))
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

	// Verify the first job's fields are parsed correctly.
	expected := []struct {
		title   string
		company string
		city    string
		state   string
	}{
		{"Senior Software Engineer", "Google", "San Francisco", "CA"},
		{"Machine Learning Engineer", "OpenAI", "New York", "NY"},
		{"Backend Developer", "Acme Corp", "Remote", ""},
	}
	for i, exp := range expected {
		if jobs[i].Title != exp.title {
			t.Errorf("job[%d] title = %q, want %q", i, jobs[i].Title, exp.title)
		}
		if jobs[i].CompanyName != exp.company {
			t.Errorf("job[%d] company = %q, want %q", i, jobs[i].CompanyName, exp.company)
		}
		if jobs[i].Location.City != exp.city {
			t.Errorf("job[%d] city = %q, want %q", i, jobs[i].Location.City, exp.city)
		}
		if jobs[i].Location.State != exp.state {
			t.Errorf("job[%d] state = %q, want %q", i, jobs[i].Location.State, exp.state)
		}
	}

	// Verify job URLs are present.
	for i, j := range jobs {
		if j.JobURL == "" {
			t.Errorf("job[%d] has empty JobURL", i)
		}
		if len(j.ID) < 5 {
			t.Errorf("job[%d] ID too short: %q", i, j.ID)
		}
	}
}

func TestJoobleFailsOn429(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	s := sut.NewWithURLs(srv.Client(), srv.URL)
	_, err := s.Scrape(context.Background(), model.ScraperInput{SearchTerm: "engineer", ResultsWanted: 1})
	if err == nil {
		t.Fatal("expected error for 429 status, got nil")
	}
}

func TestJoobleFailsOn503(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	s := sut.NewWithURLs(srv.Client(), srv.URL)
	_, err := s.Scrape(context.Background(), model.ScraperInput{SearchTerm: "engineer", ResultsWanted: 1})
	if err == nil {
		t.Fatal("expected error for 503 status, got nil")
	}
}

func TestJoobleEmptyResponse(t *testing.T) {
	html := `<html><body><div class="search-results"></div></body></html>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(html))
	}))
	defer srv.Close()

	s := sut.NewWithURLs(srv.Client(), srv.URL)
	_, err := s.Scrape(context.Background(), model.ScraperInput{SearchTerm: "nonexistent", ResultsWanted: 1})
	if err == nil {
		t.Fatal("expected error for empty response, got nil")
	}
}

func TestJoobleContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // immediately cancel

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(sampleJobHTML))
	}))
	defer srv.Close()

	s := sut.NewWithURLs(srv.Client(), srv.URL)
	_, err := s.Scrape(ctx, model.ScraperInput{SearchTerm: "engineer", ResultsWanted: 3})
	if err == nil {
		t.Fatal("expected error for cancelled context, got nil")
	}
}
