package talroo

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/arinbalyan/scrappy/internal/model"
)

const testAPIResponse = `{
  "total": 2,
  "start": 0,
  "count": 2,
  "jobs": [
    {
      "title": "Software Engineer",
      "date": "2026-05-15",
      "onclick": "https://example.com/job/123",
      "company": "TechCorp",
      "city": ["San Francisco", "Remote"],
      "coordinates": ["37.7749,-122.4194"],
      "description": "<p>Build great software.</p>"
    },
    {
      "title": "Data Analyst",
      "date": "2026-05-14",
      "onclick": "https://example.com/job/456",
      "company": "DataInc",
      "city": ["New York"],
      "coordinates": ["40.7128,-74.0060"],
      "description": "<p>Analyze data.</p>"
    },
    {
      "title": "",
      "onclick": "",
      "company": "",
      "city": [],
      "coordinates": [],
      "description": ""
    }
  ]
}`

func TestScraper_SiteName(t *testing.T) {
	s := NewWithAPIURL(nil, "", "test_id", "test_pass")
	if got := s.SiteName(); got != model.SiteTalroo {
		t.Errorf("SiteName() = %q, want %q", got, model.SiteTalroo)
	}
}

func TestScraper_Scrape(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(testAPIResponse))
	}))
	defer ts.Close()

	s := NewWithAPIURL(nil, ts.URL, "test_id", "test_pass")
	jobs, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 25})
	if err != nil {
		t.Fatalf("Scrape() returned error: %v", err)
	}

	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(jobs))
	}

	j1 := jobs[0]
	if j1.ID != "talroo-https://example.com/job/123" {
		t.Errorf("job[0].ID = %q", j1.ID)
	}
	if j1.Title != "Software Engineer" {
		t.Errorf("job[0].Title = %q, want %q", j1.Title, "Software Engineer")
	}
	if j1.CompanyName != "TechCorp" {
		t.Errorf("job[0].CompanyName = %q, want %q", j1.CompanyName, "TechCorp")
	}
	if j1.Site != string(model.SiteTalroo) {
		t.Errorf("job[0].Site = %q, want %q", j1.Site, model.SiteTalroo)
	}
	if !j1.IsRemote {
		t.Error("job[0].IsRemote should be true (city contains 'Remote')")
	}
	if j1.Location.City != "San Francisco, Remote" {
		t.Errorf("job[0].Location.City = %q, want %q", j1.Location.City, "San Francisco, Remote")
	}
	if j1.DatePosted == nil {
		t.Error("job[0].DatePosted is nil")
	}

	j2 := jobs[1]
	if j2.ID != "talroo-https://example.com/job/456" {
		t.Errorf("job[1].ID = %q", j2.ID)
	}
	if j2.Title != "Data Analyst" {
		t.Errorf("job[1].Title = %q, want %q", j2.Title, "Data Analyst")
	}
	if j2.CompanyName != "DataInc" {
		t.Errorf("job[1].CompanyName = %q, want %q", j2.CompanyName, "DataInc")
	}
	if j2.IsRemote {
		t.Error("job[1].IsRemote should be false")
	}
}

func TestScraper_Scrape_MissingCredentials(t *testing.T) {
	s := NewWithAPIURL(nil, "", "", "")
	_, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 25})
	if err == nil {
		t.Fatal("expected error for missing credentials, got nil")
	}
}

func TestScraper_Scrape_HTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	s := NewWithAPIURL(nil, ts.URL, "test_id", "test_pass")
	_, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 25})
	if err == nil {
		t.Fatal("expected error for 500 status, got nil")
	}
}

func TestScraper_Scrape_EmptyBody(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"total":0,"start":0,"count":0,"jobs":[]}`))
	}))
	defer ts.Close()

	s := NewWithAPIURL(nil, ts.URL, "test_id", "test_pass")
	_, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 25})
	if err == nil {
		t.Fatal("expected error for empty response, got nil")
	}
}

func TestNewWithAPIURL_EmptyEndpoint(t *testing.T) {
	s := NewWithAPIURL(nil, "", "test_id", "test_pass")
	if s.apiURL != apiURL {
		t.Errorf("empty endpoint should not override API URL")
	}
}
