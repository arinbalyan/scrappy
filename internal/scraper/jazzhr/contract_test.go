package jazzhr

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/arinbalyan/scrappy/internal/model"
)

const testJSON = `[
  {
    "id": "1001",
    "title": "Software Engineer",
    "city": "San Francisco",
    "state": "CA",
    "zip": "94102",
    "department": "Engineering",
    "description": "<p>Build <b>amazing</b> software.</p>",
    "type": "Full-Time",
    "original_open_date": "2026-05-15",
    "board_code": "abc123"
  },
  {
    "id": "1002",
    "title": "Product Manager",
    "city": "New York",
    "state": "NY",
    "zip": "10001",
    "department": "Product",
    "description": null,
    "type": "Full-Time",
    "original_open_date": "2026-05-10",
    "board_code": null
  },
  {
    "id": "1003",
    "title": "",
    "city": null,
    "state": null,
    "department": null,
    "description": null,
    "type": null,
    "original_open_date": null,
    "board_code": null
  }
]`

func TestScraper_SiteName(t *testing.T) {
	s := New(nil)
	if got := s.SiteName(); got != model.SiteJazzHR {
		t.Errorf("SiteName() = %q, want %q", got, model.SiteJazzHR)
	}
}

func TestScraper_Scrape(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("apikey") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(testJSON))
	}))
	defer ts.Close()

	t.Setenv("SCRAPPY_JAZZHR_SEEDS", "acme")
	t.Setenv("JAZZHR_API_KEY", "test-key-123")
	s := NewWithAPIURL(nil, ts.URL)
	jobs, err := s.Scrape(context.Background(), model.ScraperInput{
		ResultsWanted: 25,
	})
	if err != nil {
		t.Fatalf("Scrape() returned error: %v", err)
	}

	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(jobs))
	}

	// Check first job
	j0 := jobs[0]
	if j0.Title != "Software Engineer" {
		t.Errorf("job[0].Title = %q, want %q", j0.Title, "Software Engineer")
	}
	if j0.CompanyName != "acme" {
		t.Errorf("job[0].CompanyName = %q", j0.CompanyName)
	}
	if j0.Location.City != "San Francisco" {
		t.Errorf("job[0].Location.City = %q", j0.Location.City)
	}
	if j0.Location.State != "CA" {
		t.Errorf("job[0].Location.State = %q", j0.Location.State)
	}
	if j0.Description != "Build amazing software." {
		t.Errorf("job[0].Description = %q", j0.Description)
	}
	if j0.Site != string(model.SiteJazzHR) {
		t.Errorf("job[0].Site = %q", j0.Site)
	}
	if j0.Department != "Engineering" {
		t.Errorf("job[0].Department = %q", j0.Department)
	}
	if j0.JobType != "Full-Time" {
		t.Errorf("job[0].JobType = %q", j0.JobType)
	}
	if j0.DatePosted == nil {
		t.Error("job[0].DatePosted should not be nil")
	}

	// Check second job
	j1 := jobs[1]
	if j1.Title != "Product Manager" {
		t.Errorf("job[1].Title = %q", j1.Title)
	}
}

func TestScraper_Scrape_HTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	t.Setenv("JAZZHR_API_KEY", "test-key-123")
	s := NewWithAPIURL(nil, ts.URL)
	_, err := s.Scrape(context.Background(), model.ScraperInput{
		SearchTerm:    "acme",
		ResultsWanted: 25,
	})
	if err == nil {
		t.Fatal("expected error for 500 status, got nil")
	}
}

func TestScraper_Scrape_NoAPIKey(t *testing.T) {
	s := New(nil)
	_, err := s.Scrape(context.Background(), model.ScraperInput{
		SearchTerm:    "acme",
		ResultsWanted: 25,
	})
	if err == nil {
		t.Fatal("expected error when no API key set, got nil")
	}
}
