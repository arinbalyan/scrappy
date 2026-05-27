package ziprecruiter

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/arinbalyan/scrappy/internal/model"
)

const testAPIResponse = `{
  "jobs": [
    {
      "job_id": "abc123",
      "name": "Senior Software Engineer",
      "title": "Senior Software Engineer",
      "job_description": "We are looking for a senior software engineer to join our team.",
      "job_city": "San Francisco",
      "job_state": "CA",
      "job_country": "US",
      "job_url": "https://www.ziprecruiter.com/jobs/abc123",
      "apply_url": "https://apply.example.com/abc123",
      "posted_time": "2026-05-20T10:00:00Z",
      "remote": "true",
      "employment_type": "full_time",
      "salary_min_annual": 150000,
      "salary_max_annual": 200000,
      "hiring_company": {
        "name": "TechCo Inc",
        "url": "https://techco.example.com",
        "logo": "https://techco.example.com/logo.png"
      }
    },
    {
      "job_id": "def456",
      "name": "Backend Developer",
      "job_description": "Backend developer role working with Go and PostgreSQL.",
      "job_city": "New York",
      "job_state": "NY",
      "job_country": "US",
      "job_url": "",
      "url": "https://www.ziprecruiter.com/jobs/def456",
      "posted_time": "2026-05-19T15:30:00Z",
      "remote": "false",
      "salary_min_annual": 120000,
      "salary_max_annual": null,
      "hiring_company": {
        "name": "StartupXYZ"
      }
    },
    {
      "job_id": "",
      "title": "",
      "job_description": "",
      "job_city": "",
      "job_state": ""
    }
  ],
  "continue_token": null
}`

func TestScraper_SiteName(t *testing.T) {
	s := New(nil)
	if got := s.SiteName(); got != model.SiteZipRecruiter {
		t.Errorf("SiteName() = %q, want %q", got, model.SiteZipRecruiter)
	}
}

func TestScraper_Scrape(t *testing.T) {
	var requestCount int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(testAPIResponse))
	}))
	defer ts.Close()

	// Use a custom transport that redirects to the test server
	client := &http.Client{
		Transport: &testTransport{target: ts.URL},
	}

	s := &Scraper{client: client}
	jobs, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 25})
	if err != nil {
		t.Fatalf("Scrape() returned error: %v", err)
	}

	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(jobs))
	}

	j1 := jobs[0]
	if !strings.HasPrefix(j1.ID, "zr-") {
		t.Errorf("job[0].ID = %q, want prefix 'zr-'", j1.ID)
	}
	if j1.Title != "Senior Software Engineer" {
		t.Errorf("job[0].Title = %q, want %q", j1.Title, "Senior Software Engineer")
	}
	if j1.CompanyName != "TechCo Inc" {
		t.Errorf("job[0].CompanyName = %q, want %q", j1.CompanyName, "TechCo Inc")
	}
	if j1.JobURL != "https://www.ziprecruiter.com/jobs/abc123" {
		t.Errorf("job[0].JobURL = %q", j1.JobURL)
	}
	if j1.JobURLDirect != "https://apply.example.com/abc123" {
		t.Errorf("job[0].JobURLDirect = %q", j1.JobURLDirect)
	}
	if j1.IsRemote != true {
		t.Errorf("job[0].IsRemote = %v, want true", j1.IsRemote)
	}
	if j1.JobType != "fulltime" {
		t.Errorf("job[0].JobType = %q, want %q", j1.JobType, "fulltime")
	}
	if j1.Location.City != "San Francisco" {
		t.Errorf("job[0].Location.City = %q", j1.Location.City)
	}
	if j1.Location.State != "CA" {
		t.Errorf("job[0].Location.State = %q", j1.Location.State)
	}
	if j1.Compensation == nil {
		t.Fatal("job[0].Compensation is nil, expected a Compensation")
	}
	if j1.Compensation.MinAmount == nil || *j1.Compensation.MinAmount != 150000 {
		t.Errorf("job[0].Compensation.MinAmount = %v", j1.Compensation.MinAmount)
	}
	if j1.Compensation.MaxAmount == nil || *j1.Compensation.MaxAmount != 200000 {
		t.Errorf("job[0].Compensation.MaxAmount = %v", j1.Compensation.MaxAmount)
	}
	if string(j1.Compensation.Interval) != string(model.IntervalYearly) {
		t.Errorf("job[0].Compensation.Interval = %q", j1.Compensation.Interval)
	}
	if j1.Site != string(model.SiteZipRecruiter) {
		t.Errorf("job[0].Site = %q, want %q", j1.Site, model.SiteZipRecruiter)
	}
	if j1.DatePosted == nil {
		t.Error("job[0].DatePosted is nil, expected a parsed date")
	}

	j2 := jobs[1]
	if !strings.HasPrefix(j2.ID, "zr-") {
		t.Errorf("job[1].ID = %q, want prefix 'zr-'", j1.ID)
	}
	if j2.Title != "Backend Developer" {
		t.Errorf("job[1].Title = %q, want %q", j2.Title, "Backend Developer")
	}
	if j2.CompanyName != "StartupXYZ" {
		t.Errorf("job[1].CompanyName = %q, want %q", j2.CompanyName, "StartupXYZ")
	}
	// Should fall back to `url` when `job_url` is empty
	if !strings.Contains(j2.JobURL, "def456") {
		t.Errorf("job[1].JobURL = %q, should contain fallback URL", j2.JobURL)
	}
	if j2.Compensation.MaxAmount != nil {
		t.Errorf("job[1].Compensation.MaxAmount = %v, want nil", j2.Compensation.MaxAmount)
	}
}

func TestScraper_Scrape_HTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	client := &http.Client{
		Transport: &testTransport{target: ts.URL},
	}
	s := &Scraper{client: client}
	_, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 25})
	if err == nil {
		t.Fatal("expected error for 500 status, got nil")
	}
}

func TestScraper_Scrape_EmptyResponse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"jobs":[],"continue_token":null}`))
	}))
	defer ts.Close()

	client := &http.Client{
		Transport: &testTransport{target: ts.URL},
	}
	s := &Scraper{client: client}
	_, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 25})
	if err == nil {
		t.Fatal("expected error for empty response, got nil")
	}
}

// testTransport rewrites requests to point at a test server.
type testTransport struct {
	target string
}

func (t *testTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Rewrite the request URL to the test server
	newURL := t.target + req.URL.Path
	if req.URL.RawQuery != "" {
		newURL += "?" + req.URL.RawQuery
	}
	newReq, _ := http.NewRequestWithContext(req.Context(), req.Method, newURL, req.Body)
	newReq.Header = req.Header
	return http.DefaultTransport.RoundTrip(newReq)
}
