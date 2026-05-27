package manatal

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/arinbalyan/scrappy/internal/model"
)

const testJSON = `{
  "count": 2,
  "next": null,
  "results": [
    {
      "id": 301,
      "position_name": "Senior Data Scientist",
      "description": "<p>Build <b>ML</b> models.</p>",
      "requirement": "5+ years experience",
      "department": "Data",
      "location": {
        "city": "San Francisco",
        "state": "CA",
        "country": "USA"
      },
      "employment_type": "Full-Time",
      "salary_min": 180000,
      "salary_max": 250000,
      "salary_currency": "USD",
      "created_at": "2026-05-20T08:00:00Z",
      "updated_at": "2026-05-20T08:00:00Z",
      "apply_url": "https://api.manatal.com/open/v1/career-page/acme/jobs/301/apply",
      "career_page_url": null
    },
    {
      "id": 302,
      "position_name": "DevOps Engineer",
      "description": "<p>Manage <b>infrastructure</b>.</p>",
      "requirement": null,
      "department": null,
      "location": null,
      "employment_type": null,
      "salary_min": null,
      "salary_max": null,
      "salary_currency": null,
      "created_at": "2026-05-18",
      "updated_at": "2026-05-18",
      "apply_url": null,
      "career_page_url": null
    },
    {
      "id": 303,
      "position_name": "",
      "description": "",
      "department": null,
      "location": null,
      "salary_min": null,
      "salary_max": null,
      "created_at": ""
    }
  ]
}`

func TestScraper_SiteName(t *testing.T) {
	s := New(nil)
	if got := s.SiteName(); got != model.SiteManatal {
		t.Errorf("SiteName() = %q, want %q", got, model.SiteManatal)
	}
}

func TestScraper_Scrape(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(testJSON))
	}))
	defer ts.Close()

	t.Setenv("SCRAPPY_MANATAL_SEEDS", "acme")
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
	if j0.Title != "Senior Data Scientist" {
		t.Errorf("job[0].Title = %q, want %q", j0.Title, "Senior Data Scientist")
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
	if j0.Location.Country != "USA" {
		t.Errorf("job[0].Location.Country = %q", j0.Location.Country)
	}
	if j0.Description != "Build ML models." {
		t.Errorf("job[0].Description = %q", j0.Description)
	}
	if j0.Site != string(model.SiteManatal) {
		t.Errorf("job[0].Site = %q", j0.Site)
	}
	if j0.Department != "Data" {
		t.Errorf("job[0].Department = %q", j0.Department)
	}
	if j0.Compensation == nil {
		t.Fatal("job[0].Compensation should not be nil")
	}
	if j0.Compensation.MinAmount == nil || *j0.Compensation.MinAmount != 180000 {
		t.Errorf("job[0].Compensation.MinAmount = %v", j0.Compensation.MinAmount)
	}

	// Check second job (no location)
	j1 := jobs[1]
	if j1.Title != "DevOps Engineer" {
		t.Errorf("job[1].Title = %q", j1.Title)
	}
	if j1.Compensation != nil {
		t.Error("job[1].Compensation should be nil")
	}
	if j1.JobURL == "" {
		t.Error("job[1].JobURL should have fallback URL")
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
