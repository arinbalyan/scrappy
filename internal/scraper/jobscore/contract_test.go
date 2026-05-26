package jobscore

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/arinbalyan/scrappy/internal/model"
)

const testJSON = `[
  {
    "id": "101",
    "title": "Senior Frontend Engineer",
    "detail_url": "https://careers.jobscore.com/acme/101",
    "description": "<p>Build <b>React</b> components.</p>",
    "department": "Engineering",
    "location": {
      "city": "San Francisco",
      "state": "CA",
      "country": "USA"
    },
    "created_at": "2026-05-20T10:00:00Z"
  },
  {
    "id": "102",
    "title": "Remote DevOps Engineer",
    "detail_url": null,
    "description": null,
    "department": "Infrastructure",
    "location": {
      "city": "Remote",
      "state": null,
      "country": null
    },
    "created_at": "2026-05-18T08:00:00Z"
  },
  {
    "id": "103",
    "title": "",
    "detail_url": null,
    "description": null,
    "department": null,
    "location": null,
    "created_at": null
  }
]`

func TestScraper_SiteName(t *testing.T) {
	s := New(nil)
	if got := s.SiteName(); got != model.SiteJobScore {
		t.Errorf("SiteName() = %q, want %q", got, model.SiteJobScore)
	}
}

func TestScraper_Scrape(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(testJSON))
	}))
	defer ts.Close()

	t.Setenv("SCRAPPY_JOBSCORE_SEEDS", "acme")
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
	if j0.Title != "Senior Frontend Engineer" {
		t.Errorf("job[0].Title = %q, want %q", j0.Title, "Senior Frontend Engineer")
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
	if j0.Description != "Build React components." {
		t.Errorf("job[0].Description = %q", j0.Description)
	}
	if j0.Site != string(model.SiteJobScore) {
		t.Errorf("job[0].Site = %q", j0.Site)
	}
	if j0.Department != "Engineering" {
		t.Errorf("job[0].Department = %q", j0.Department)
	}

	// Check second job (remote, no detail_url)
	j1 := jobs[1]
	if j1.Title != "Remote DevOps Engineer" {
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

func TestScraper_Scrape_SearchTerm(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(testJSON))
	}))
	defer ts.Close()

	t.Setenv("SCRAPPY_JOBSCORE_SEEDS", "acme")
	s := NewWithAPIURL(nil, ts.URL)
	jobs, err := s.Scrape(context.Background(), model.ScraperInput{
		SearchTerm:    "devops",
		ResultsWanted: 25,
	})
	if err != nil {
		t.Fatalf("Scrape() returned error: %v", err)
	}

	if len(jobs) != 1 {
		t.Fatalf("expected 1 job matching 'devops', got %d", len(jobs))
	}
	if jobs[0].Title != "Remote DevOps Engineer" {
		t.Errorf("job[0].Title = %q", jobs[0].Title)
	}
}
