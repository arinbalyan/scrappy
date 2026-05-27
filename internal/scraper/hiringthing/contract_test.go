package hiringthing

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/arinbalyan/scrappy/internal/model"
)

const testJSON = `{
  "jobs": [
    {
      "id": 12345,
      "title": "Senior Software Engineer",
      "description": "<p>Build great software at <b>Acme Corp</b>.</p>",
      "location": "San Francisco, CA",
      "department": "Engineering",
      "type": "Full-Time",
      "created_at": "2026-05-15T10:00:00Z",
      "url": "https://jobs.hiringthing.com/12345",
      "company_name": "Acme Corp",
      "status": "open",
      "salary": "$150k - $200k",
      "experience": "5+ years"
    },
    {
      "id": 67890,
      "title": "Product Manager",
      "description": null,
      "location": "Remote",
      "department": "Product",
      "type": "Full-Time",
      "created_at": "2026-05-14T08:00:00Z",
      "url": null,
      "company_name": "Acme Corp",
      "status": "open",
      "salary": null,
      "experience": null
    },
    {
      "id": 99999,
      "title": "",
      "description": "<p>Empty title</p>",
      "location": "Nowhere",
      "department": null,
      "type": null,
      "created_at": null,
      "url": null,
      "company_name": null,
      "status": null,
      "salary": null,
      "experience": null
    }
  ]
}`

func TestScraper_SiteName(t *testing.T) {
	s := New(nil)
	if got := s.SiteName(); got != model.SiteHiringThing {
		t.Errorf("SiteName() = %q, want %q", got, model.SiteHiringThing)
	}
}

func TestScraper_Scrape(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(testJSON))
	}))
	defer ts.Close()

	s := NewWithAPIURL(nil, ts.URL)
	jobs, err := s.Scrape(context.Background(), model.ScraperInput{
		SearchTerm:    "acme",
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
	if j0.Title != "Senior Software Engineer" {
		t.Errorf("job[0].Title = %q, want %q", j0.Title, "Senior Software Engineer")
	}
	if j0.CompanyName != "Acme Corp" {
		t.Errorf("job[0].CompanyName = %q, want %q", j0.CompanyName, "Acme Corp")
	}
	if j0.Location.City != "San Francisco, CA" {
		t.Errorf("job[0].Location.City = %q", j0.Location.City)
	}
	if j0.Description != "Build great software at Acme Corp." {
		t.Errorf("job[0].Description = %q", j0.Description)
	}
	if j0.Site != string(model.SiteHiringThing) {
		t.Errorf("job[0].Site = %q", j0.Site)
	}

	// Check second job
	j1 := jobs[1]
	if j1.Title != "Product Manager" {
		t.Errorf("job[1].Title = %q", j1.Title)
	}
	if j1.Location.City != "Remote" {
		t.Errorf("job[1].Location.City = %q", j1.Location.City)
	}
	if j1.Description != "" {
		t.Errorf("job[1].Description should be empty")
	}
}

func TestScraper_Scrape_HTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	s := NewWithAPIURL(nil, ts.URL)
	_, err := s.Scrape(context.Background(), model.ScraperInput{
		SearchTerm:    "acme",
		ResultsWanted: 25,
	})
	if err == nil {
		t.Fatal("expected error for 500 status, got nil")
	}
}

func TestNewWithAPIURL_EmptyEndpoint(t *testing.T) {
	s1 := NewWithAPIURL(nil, "")
	s2 := New(nil)
	if s1.apiURL != s2.apiURL {
		t.Errorf("empty endpoint should not override api URL")
	}
}
