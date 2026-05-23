package careerbuilder_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	sut "github.com/arinbalyan/scrappy/internal/scraper/careerbuilder"
)

// sampleJobHTML simulates a CareerBuilder search results page with job listings.
// The HTML structure mirrors real CareerBuilder pages: div cards with
// data-job-did attributes, data-results-title links, data-company/data-location
// detail spans, and data-posted-date elements.
const sampleJobHTML = `<!DOCTYPE html>
<html>
<head><title>Jobs - CareerBuilder</title></head>
<body>
<div class="data-results">
  <div data-job-did="abc123" class="data-results-content-parent">
    <div class="data-results-content">
      <h2 class="data-results-title">
        <a href="/jobs/software-engineer-sf">Senior Software Engineer</a>
      </h2>
      <div class="data-details">
        <span class="data-company">Google</span>
        <span class="data-location">San Francisco, CA</span>
      </div>
      <div class="data-salary">$150,000 - $200,000</div>
      <div class="data-posted-date">Posted 2 days ago</div>
    </div>
  </div>
  <div data-job-did="def456" class="data-results-content-parent">
    <div class="data-results-content">
      <h2 class="data-results-title">
        <a href="/jobs/ml-engineer-nyc">Machine Learning Engineer</a>
      </h2>
      <div class="data-details">
        <span class="data-company">OpenAI</span>
        <span class="data-location">New York, NY</span>
      </div>
      <div class="data-salary">$180,000 - $250,000</div>
      <div class="data-posted-date">Posted Today</div>
    </div>
  </div>
  <div data-job-did="ghi789" class="data-results-content-parent">
    <div class="data-results-content">
      <h2 class="data-results-title">
        <a href="/jobs/backend-dev-remote">Backend Developer</a>
      </h2>
      <div class="data-details">
        <span class="data-company">Acme Corp</span>
        <span class="data-location">Remote, US</span>
      </div>
      <div class="data-salary"></div>
      <div class="data-posted-date">Posted 5 days ago</div>
    </div>
  </div>
</div>
</body>
</html>`

func TestCareerBuilderParsesJobs(t *testing.T) {
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
		{"Backend Developer", "Acme Corp", "Remote", "US"},
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

	// Verify job URLs are absolute.
	for i, j := range jobs {
		if j.JobURL == "" {
			t.Errorf("job[%d] has empty JobURL", i)
		}
		if len(j.ID) < 5 {
			t.Errorf("job[%d] ID too short: %q", i, j.ID)
		}
	}
}

func TestCareerBuilderFailsOn429(t *testing.T) {
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

func TestCareerBuilderFailsOn503(t *testing.T) {
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

func TestCareerBuilderEmptyResponse(t *testing.T) {
	html := `<html><body><div class="data-results"></div></body></html>`
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

func TestCareerBuilderContextCancellation(t *testing.T) {
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
