package homerun

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/arinbalyan/scrappy/internal/model"
)

const testJSON = `{
  "data": [
    {
      "id": 1001,
      "title": "Senior Backend Engineer",
      "description": "<p>Build APIs with <b>Go</b> and Python.</p>",
      "location": "Amsterdam",
      "department": "Engineering",
      "employment_type": "Full-Time",
      "application_url": "https://app.homerun.co/acme/senior-backend-engineer",
      "slug": "senior-backend-engineer",
      "created_at": "2026-05-20T08:00:00Z",
      "updated_at": "2026-05-20T08:00:00Z",
      "status": "published"
    },
    {
      "id": 1002,
      "title": "Remote UX Designer",
      "description": null,
      "location": "Remote",
      "department": "Design",
      "employment_type": "Contract",
      "application_url": null,
      "slug": "remote-ux-designer",
      "created_at": "2026-05-19T10:00:00Z",
      "status": "published"
    },
    {
      "id": 1003,
      "title": "",
      "description": "<p>Empty title job</p>",
      "location": "Nowhere"
    }
  ]
}`

func TestScraper_SiteName(t *testing.T) {
	s := New(nil)
	if got := s.SiteName(); got != model.SiteHomerun {
		t.Errorf("SiteName() = %q, want %q", got, model.SiteHomerun)
	}
}

func TestScraper_Scrape(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(testJSON))
	}))
	defer ts.Close()

	t.Setenv("SCRAPPY_HOMERUN_SEEDS", "acme")
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
	if j0.Title != "Senior Backend Engineer" {
		t.Errorf("job[0].Title = %q, want %q", j0.Title, "Senior Backend Engineer")
	}
	if j0.CompanyName != "acme" {
		t.Errorf("job[0].CompanyName = %q", j0.CompanyName)
	}
	if j0.Location.City != "Amsterdam" {
		t.Errorf("job[0].Location.City = %q", j0.Location.City)
	}
	if j0.Description != "Build APIs with Go and Python." {
		t.Errorf("job[0].Description = %q", j0.Description)
	}
	if j0.IsRemote {
		t.Error("job[0].IsRemote should be false")
	}
	if j0.Site != string(model.SiteHomerun) {
		t.Errorf("job[0].Site = %q", j0.Site)
	}

	// Check second job (remote)
	j1 := jobs[1]
	if j1.Title != "Remote UX Designer" {
		t.Errorf("job[1].Title = %q", j1.Title)
	}
	if !j1.IsRemote {
		t.Error("job[1].IsRemote should be true")
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

	s := NewWithAPIURL(nil, ts.URL)
	jobs, err := s.Scrape(context.Background(), model.ScraperInput{
		SearchTerm:    "ux",
		ResultsWanted: 25,
	})
	if err != nil {
		t.Fatalf("Scrape() returned error: %v", err)
	}

	if len(jobs) != 1 {
		t.Fatalf("expected 1 job matching 'ux', got %d", len(jobs))
	}
	if jobs[0].Title != "Remote UX Designer" {
		t.Errorf("job[0].Title = %q", jobs[0].Title)
	}
}
