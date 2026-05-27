package jobvite

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/arinbalyan/scrappy/internal/model"
)

const testJSON = `{
  "requisitions": [
    {
      "eId": "J001",
      "title": "Senior Software Engineer",
      "department": "Engineering",
      "category": "Tech",
      "location": "San Francisco, CA",
      "city": "San Francisco",
      "state": "CA",
      "country": "USA",
      "type": "Full-Time",
      "date": "2026-05-15",
      "description": "<p>Build <b>scalable</b> systems.</p>",
      "briefDescription": "Systems engineering role",
      "applyUrl": "https://jobs.jobvite.com/acme/apply/J001",
      "detailUrl": null,
      "requisitionId": "J001"
    },
    {
      "eId": "J002",
      "title": "Remote Technical Writer",
      "department": "Docs",
      "category": null,
      "location": "Remote",
      "city": null,
      "state": null,
      "country": null,
      "type": "Contract",
      "date": "2026-05-10",
      "description": null,
      "briefDescription": null,
      "applyUrl": null,
      "detailUrl": null,
      "requisitionId": "J002"
    },
    {
      "eId": "J003",
      "title": "",
      "department": null,
      "category": null,
      "location": null,
      "city": null,
      "state": null,
      "country": null,
      "type": null,
      "date": null,
      "description": null,
      "briefDescription": null,
      "applyUrl": null,
      "detailUrl": null,
      "requisitionId": "J003"
    }
  ],
  "total": 2
}`

func TestScraper_SiteName(t *testing.T) {
	s := New(nil)
	if got := s.SiteName(); got != model.SiteJobvite {
		t.Errorf("SiteName() = %q, want %q", got, model.SiteJobvite)
	}
}

func TestScraper_Scrape(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(testJSON))
	}))
	defer ts.Close()

	t.Setenv("SCRAPPY_JOBVITE_SEEDS", "acme")
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
	if j0.Title != "Senior Software Engineer" {
		t.Errorf("job[0].Title = %q, want %q", j0.Title, "Senior Software Engineer")
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
	if j0.Description != "Build scalable systems." {
		t.Errorf("job[0].Description = %q", j0.Description)
	}
	if j0.Site != string(model.SiteJobvite) {
		t.Errorf("job[0].Site = %q", j0.Site)
	}
	if j0.Department != "Engineering" {
		t.Errorf("job[0].Department = %q", j0.Department)
	}
	if j0.JobType != "Full-Time" {
		t.Errorf("job[0].JobType = %q", j0.JobType)
	}

	// Check second job (remote)
	j1 := jobs[1]
	if j1.Title != "Remote Technical Writer" {
		t.Errorf("job[1].Title = %q", j1.Title)
	}
	if !j1.IsRemote {
		t.Error("job[1].IsRemote should be true")
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
