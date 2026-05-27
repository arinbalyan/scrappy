package loxo

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/arinbalyan/scrappy/internal/model"
)

const testJSON = `[
  {
    "id": 1001,
    "title": "Senior Backend Engineer",
    "description": "<p>Build <b>APIs</b> with Go.</p>",
    "location": "San Francisco, CA",
    "department": "Engineering",
    "type": "Full-Time",
    "employment_type": "Full-Time",
    "created_at": "2026-05-20T10:00:00Z",
    "url": "https://app.loxo.co/acme/jobs/1001",
    "apply_url": "https://app.loxo.co/acme/jobs/1001/apply",
    "remote": false,
    "salary": {
      "min": 150000,
      "max": 200000,
      "currency": "USD",
      "interval": "yearly"
    },
    "category": "Engineering",
    "company_name": "Acme Corp"
  },
  {
    "id": 1002,
    "title": "Remote Product Designer",
    "description": null,
    "location": {"city": "Remote", "state": null, "country": null},
    "department": null,
    "type": null,
    "employment_type": "Contract",
    "created_at": "2026-05-18",
    "url": null,
    "apply_url": null,
    "remote": true,
    "salary": null,
    "category": null,
    "company_name": null
  },
  {
    "id": 1003,
    "title": "",
    "description": null,
    "location": null,
    "department": null,
    "type": null,
    "employment_type": null,
    "created_at": null,
    "url": null,
    "apply_url": null,
    "salary": null,
    "category": null,
    "company_name": null
  }
]`

func TestScraper_SiteName(t *testing.T) {
	s := New(nil)
	if got := s.SiteName(); got != model.SiteLoxo {
		t.Errorf("SiteName() = %q, want %q", got, model.SiteLoxo)
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
	if j0.Title != "Senior Backend Engineer" {
		t.Errorf("job[0].Title = %q, want %q", j0.Title, "Senior Backend Engineer")
	}
	if j0.CompanyName != "Acme Corp" {
		t.Errorf("job[0].CompanyName = %q, want %q", j0.CompanyName, "Acme Corp")
	}
	if j0.Location.City != "San Francisco, CA" {
		t.Errorf("job[0].Location.City = %q", j0.Location.City)
	}
	if j0.Description != "Build APIs with Go." {
		t.Errorf("job[0].Description = %q", j0.Description)
	}
	if j0.Site != string(model.SiteLoxo) {
		t.Errorf("job[0].Site = %q", j0.Site)
	}
	if j0.Department != "Engineering" {
		t.Errorf("job[0].Department = %q", j0.Department)
	}
	if j0.JobType != "Full-Time" {
		t.Errorf("job[0].JobType = %q", j0.JobType)
	}
	if j0.IsRemote {
		t.Error("job[0].IsRemote should be false")
	}
	if j0.Compensation == nil {
		t.Fatal("job[0].Compensation should not be nil")
	}
	if j0.Compensation.MinAmount == nil || *j0.Compensation.MinAmount != 150000 {
		t.Errorf("job[0].Compensation.MinAmount = %v", j0.Compensation.MinAmount)
	}
	if j0.Compensation.MaxAmount == nil || *j0.Compensation.MaxAmount != 200000 {
		t.Errorf("job[0].Compensation.MaxAmount = %v", j0.Compensation.MaxAmount)
	}
	if j0.Compensation.Currency != "USD" {
		t.Errorf("job[0].Compensation.Currency = %q", j0.Compensation.Currency)
	}
	if j0.Compensation.Interval != model.IntervalYearly {
		t.Errorf("job[0].Compensation.Interval = %q", j0.Compensation.Interval)
	}

	// Check second job (remote, structured location)
	j1 := jobs[1]
	if j1.Title != "Remote Product Designer" {
		t.Errorf("job[1].Title = %q", j1.Title)
	}
	if !j1.IsRemote {
		t.Error("job[1].IsRemote should be true")
	}
	if j1.Location.City != "Remote" {
		t.Errorf("job[1].Location.City = %q", j1.Location.City)
	}
	if j1.JobURL == "" {
		t.Error("job[1].JobURL should have fallback URL")
	}
	if j1.CompanyName != "acme" {
		t.Errorf("job[1].CompanyName = %q, want fallback 'acme'", j1.CompanyName)
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
