package powertofly

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/arinbalyan/scrappy/internal/model"
)

const testAPIResponse = `{
  "items": [
    {
      "title": "Senior Product Designer",
      "description": "DesignCo",
      "link": "https://powertofly.com/jobs/senior-product-designer-123",
      "job_location": "San Francisco, United States",
      "published_on": "2026-05-15T10:00:00Z",
      "categories": ["Design", "Product"],
      "type": "Full-time",
      "guid": "https://powertofly.com/jobs/senior-product-designer-123"
    },
    {
      "title": "Remote Software Engineer",
      "description": "TechRemote Inc",
      "link": "https://powertofly.com/jobs/remote-software-engineer-456",
      "job_location": "Remote, United States",
      "published_on": "2026-05-16T14:30:00Z",
      "categories": ["Engineering"],
      "type": "Remote",
      "guid": "https://powertofly.com/jobs/remote-software-engineer-456"
    },
    {
      "title": "",
      "description": "NoName",
      "link": "https://powertofly.com/jobs/empty-title",
      "job_location": "",
      "published_on": "",
      "categories": [],
      "type": "",
      "guid": ""
    }
  ],
  "status": "ok"
}`

func TestScraper_SiteName(t *testing.T) {
	s := New(nil)
	if got := s.SiteName(); got != model.SitePowerToFly {
		t.Errorf("SiteName() = %q, want %q", got, model.SitePowerToFly)
	}
}

func TestScraper_Scrape(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(testAPIResponse))
	}))
	defer ts.Close()

	s := NewWithAPIURL(nil, ts.URL)
	jobs, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 25})
	if err != nil {
		t.Fatalf("Scrape() returned error: %v", err)
	}

	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(jobs))
	}

	j1 := jobs[0]
	if j1.ID != "powertofly-senior-product-designer-123" {
		t.Errorf("job[0].ID = %q, want %q", j1.ID, "powertofly-senior-product-designer-123")
	}
	if j1.Title != "Senior Product Designer" {
		t.Errorf("job[0].Title = %q, want %q", j1.Title, "Senior Product Designer")
	}
	if j1.CompanyName != "DesignCo" {
		t.Errorf("job[0].CompanyName = %q, want %q", j1.CompanyName, "DesignCo")
	}
	if j1.JobURL != "https://powertofly.com/jobs/senior-product-designer-123" {
		t.Errorf("job[0].JobURL = %q", j1.JobURL)
	}
	if j1.Site != string(model.SitePowerToFly) {
		t.Errorf("job[0].Site = %q, want %q", j1.Site, model.SitePowerToFly)
	}
	if j1.IsRemote != false {
		t.Error("job[0].IsRemote should be false")
	}
	if j1.Location.City != "San Francisco" {
		t.Errorf("job[0].Location.City = %q, want %q", j1.Location.City, "San Francisco")
	}
	if j1.Location.Country != "United States" {
		t.Errorf("job[0].Location.Country = %q, want %q", j1.Location.Country, "United States")
	}
	if j1.DatePosted == nil {
		t.Error("job[0].DatePosted is nil, expected a parsed date")
	}

	j2 := jobs[1]
	if j2.ID != "powertofly-remote-software-engineer-456" {
		t.Errorf("job[1].ID = %q, want %q", j2.ID, "powertofly-remote-software-engineer-456")
	}
	if j2.Title != "Remote Software Engineer" {
		t.Errorf("job[1].Title = %q, want %q", j2.Title, "Remote Software Engineer")
	}
	if j2.IsRemote != true {
		t.Error("job[1].IsRemote should be true")
	}
	if j2.Location.City != "Remote" {
		t.Errorf("job[1].Location.City = %q, want %q", j2.Location.City, "Remote")
	}
}

func TestScraper_Scrape_SearchTerm(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(testAPIResponse))
	}))
	defer ts.Close()

	s := NewWithAPIURL(nil, ts.URL)
	jobs, err := s.Scrape(context.Background(), model.ScraperInput{
		SearchTerm:    "engineering",
		ResultsWanted: 25,
	})
	if err != nil {
		t.Fatalf("Scrape() returned error: %v", err)
	}

	if len(jobs) != 1 {
		t.Fatalf("expected 1 job matching 'engineering', got %d", len(jobs))
	}
	if jobs[0].Title != "Remote Software Engineer" {
		t.Errorf("job[0].Title = %q, want %q", jobs[0].Title, "Remote Software Engineer")
	}
}

func TestScraper_Scrape_HTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer ts.Close()

	s := NewWithAPIURL(nil, ts.URL)
	_, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 25})
	if err == nil {
		t.Fatal("expected error for 429 status, got nil")
	}
}

func TestScraper_Scrape_EmptyBody(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"items":[],"status":"ok"}`))
	}))
	defer ts.Close()

	s := NewWithAPIURL(nil, ts.URL)
	_, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 25})
	if err == nil {
		t.Fatal("expected error for empty response, got nil")
	}
}

func TestNewWithAPIURL_EmptyEndpoint(t *testing.T) {
	s1 := NewWithAPIURL(nil, "")
	s2 := New(nil)
	if s1.apiURL != s2.apiURL {
		t.Errorf("empty endpoint should not override API URL")
	}
}
