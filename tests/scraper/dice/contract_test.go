package dice_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	sut "github.com/arinbalyan/scrappy/internal/scraper/dice"
)

// sampleAPIResponse simulates a Dice REST API response with job listings.
// The JSON structure mirrors real Dice API output.
var sampleAPIResponse = `{
  "data": [
    {
      "id": "job-001",
      "title": "Senior Software Engineer",
      "companyName": "Google",
      "summary": "Build and maintain large-scale distributed systems.",
      "detailsPageUrl": "https://www.dice.com/job-detail/job-001",
      "formattedLocation": "San Francisco, CA",
      "postedDate": "2026-05-20T12:00:00Z",
      "employmentType": "full-time",
      "isRemote": false,
      "payRateRange": {
        "min": 150000,
        "max": 200000
      }
    },
    {
      "id": "job-002",
      "title": "Machine Learning Engineer",
      "companyName": "OpenAI",
      "summary": "Work on cutting-edge AI models.",
      "detailsPageUrl": "https://www.dice.com/job-detail/job-002",
      "formattedLocation": "Remote, US",
      "postedDate": "2026-05-19T08:00:00Z",
      "employmentType": "full-time",
      "isRemote": true
    },
    {
      "id": "job-003",
      "title": "Backend Developer",
      "companyName": "Acme Corp",
      "detailsPageUrl": "https://www.dice.com/job-detail/job-003",
      "formattedLocation": "New York, NY",
      "postedDate": "2026-05-18T10:30:00Z",
      "employmentType": "contract",
      "isRemote": false,
      "salary": "$120,000 - $160,000"
    }
  ],
  "meta": {
    "totalHits": 3,
    "page": 1,
    "pageSize": 20
  }
}`

// sampleHTMLResponse simulates a Dice HTML search results page with job cards.
// The HTML mirrors Dice's data-cy attribute structure.
const sampleHTMLResponse = `<!DOCTYPE html>
<html>
<head><title>Jobs - Dice</title></head>
<body>
<div id="search-results">
  <div data-cy="search-card" class="card">
    <div class="card-header">
      <a data-cy="card-title" href="/job-detail/html-001">Senior Frontend Engineer</a>
    </div>
    <div class="card-body">
      <div data-cy="search-result-company-name">Meta</div>
      <div data-cy="search-result-location">Austin, TX</div>
      <div data-cy="search-result-salary">$160,000 - $200,000</div>
      <div data-cy="card-posted-date">Posted 3 days ago</div>
    </div>
  </div>
  <div data-cy="search-card" class="card">
    <div class="card-header">
      <a data-cy="card-title" href="/job-detail/html-002">DevOps Engineer</a>
    </div>
    <div class="card-body">
      <div data-cy="search-result-company-name">Amazon</div>
      <div data-cy="search-result-location">Seattle, WA</div>
      <div data-cy="card-posted-date">Posted 1 week ago</div>
    </div>
  </div>
  <div data-cy="search-card" class="card">
    <div class="card-header">
      <a data-cy="card-title" href="/job-detail/html-003">Data Scientist</a>
    </div>
    <div class="card-body">
      <div data-cy="search-result-company-name">Netflix</div>
      <div data-cy="search-result-location">Remote, US</div>
      <div data-cy="card-posted-date">Posted Today</div>
    </div>
  </div>
</div>
</body>
</html>`

func TestDiceHappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "job-search-api") || strings.Contains(r.URL.Path, "search") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(sampleAPIResponse))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	s := sut.NewWithURLs(srv.Client(), srv.URL+"/v1/dice/jobs/search", srv.URL+"/jobs")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	jobs, err := s.Scrape(ctx, model.ScraperInput{SearchTerm: "engineer", ResultsWanted: 3})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(jobs) != 3 {
		t.Fatalf("expected 3 jobs, got %d", len(jobs))
	}

	// Verify first job fields.
	if jobs[0].Title != "Senior Software Engineer" {
		t.Errorf("job[0] title = %q, want %q", jobs[0].Title, "Senior Software Engineer")
	}
	if jobs[0].CompanyName != "Google" {
		t.Errorf("job[0] company = %q, want %q", jobs[0].CompanyName, "Google")
	}
	if jobs[0].Location.City != "San Francisco" {
		t.Errorf("job[0] city = %q, want %q", jobs[0].Location.City, "San Francisco")
	}
	if jobs[0].IsRemote {
		t.Errorf("job[0] should not be remote")
	}
	if jobs[0].Compensation == nil {
		t.Errorf("job[0] compensation should not be nil")
	}

	// Verify job URLs are valid.
	for i, j := range jobs {
		if j.JobURL == "" {
			t.Errorf("job[%d] has empty JobURL", i)
		}
		if len(j.ID) < 5 {
			t.Errorf("job[%d] ID too short: %q", i, j.ID)
		}
	}
}

func TestDiceFailsOn429(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	s := sut.NewWithURLs(srv.Client(), srv.URL+"/v1/dice/jobs/search", srv.URL+"/jobs")
	_, err := s.Scrape(context.Background(), model.ScraperInput{SearchTerm: "engineer", ResultsWanted: 1})
	if err == nil {
		t.Fatal("expected error for 429 status, got nil")
	}
}

func TestDiceFailsOn503(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	s := sut.NewWithURLs(srv.Client(), srv.URL+"/v1/dice/jobs/search", srv.URL+"/jobs")
	_, err := s.Scrape(context.Background(), model.ScraperInput{SearchTerm: "engineer", ResultsWanted: 1})
	if err == nil {
		t.Fatal("expected error for 503 status, got nil")
	}
}

func TestDiceEmptyResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[],"meta":{"totalHits":0,"page":1,"pageSize":20}}`))
	}))
	defer srv.Close()

	s := sut.NewWithURLs(srv.Client(), srv.URL+"/v1/dice/jobs/search", srv.URL+"/jobs")
	_, err := s.Scrape(context.Background(), model.ScraperInput{SearchTerm: "nonexistent", ResultsWanted: 1})
	if err == nil {
		t.Fatal("expected error for empty response, got nil")
	}
}

func TestDiceContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // immediately cancel

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(sampleAPIResponse))
	}))
	defer srv.Close()

	s := sut.NewWithURLs(srv.Client(), srv.URL+"/v1/dice/jobs/search", srv.URL+"/jobs")
	_, err := s.Scrape(ctx, model.ScraperInput{SearchTerm: "engineer", ResultsWanted: 3})
	if err == nil {
		t.Fatal("expected error for cancelled context, got nil")
	}
}

func TestDiceHTMLFallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// API returns empty -> triggers HTML fallback
		if strings.Contains(r.URL.Path, "search") && strings.Contains(r.URL.RawQuery, "countryCode2") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data":[],"meta":{"totalHits":0,"page":1,"pageSize":20}}`))
			return
		}
		// HTML endpoint
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(sampleHTMLResponse))
	}))
	defer srv.Close()

	s := sut.NewWithURLs(srv.Client(), srv.URL+"/v1/dice/jobs/search", srv.URL+"/jobs")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	jobs, err := s.Scrape(ctx, model.ScraperInput{SearchTerm: "engineer", ResultsWanted: 3})
	if err != nil {
		t.Fatalf("unexpected error on HTML fallback: %v", err)
	}
	if len(jobs) != 3 {
		t.Fatalf("expected 3 jobs from HTML fallback, got %d", len(jobs))
	}

	// Verify HTML-fallback jobs.
	if jobs[0].Title != "Senior Frontend Engineer" {
		t.Errorf("job[0] title = %q, want %q", jobs[0].Title, "Senior Frontend Engineer")
	}
	if jobs[0].CompanyName != "Meta" {
		t.Errorf("job[0] company = %q, want %q", jobs[0].CompanyName, "Meta")
	}
}

// TestDiceAPIResponseRoundtrip verifies the JSON unmarshalling directly.
func TestDiceAPIResponseRoundtrip(t *testing.T) {
	var resp struct {
		Data []struct {
			ID               string  `json:"id"`
			Title            string  `json:"title"`
			CompanyName      string  `json:"companyName"`
			FormattedLocation string `json:"formattedLocation"`
			Salary           string  `json:"salary"`
			IsRemote         bool    `json:"isRemote"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(sampleAPIResponse), &resp); err != nil {
		t.Fatalf("failed to unmarshal sample response: %v", err)
	}
	if len(resp.Data) != 3 {
		t.Fatalf("expected 3 jobs in sample, got %d", len(resp.Data))
	}
	if resp.Data[0].Title != "Senior Software Engineer" {
		t.Errorf("title = %q, want %q", resp.Data[0].Title, "Senior Software Engineer")
	}
}
