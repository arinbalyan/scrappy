package ismartrecruit

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/arinbalyan/scrappy/internal/model"
)

const testJSON = `[
  {
    "jobId": "job-001",
    "jobTitle": "Senior Backend Engineer",
    "city": "San Francisco",
    "country": "USA",
    "jobCategory": "Engineering",
    "datePosted": "2026-05-20",
    "description": "<p>Build scalable <b>APIs</b> with Go.</p>",
    "applyUrl": "https://app.ismartrecruit.com/apply/job-001",
    "companyName": "Acme Corp"
  },
  {
    "jobId": "job-002",
    "jobTitle": "Remote UX Designer",
    "city": "Remote",
    "country": null,
    "jobCategory": "Design",
    "datePosted": "2026-05-19",
    "description": null,
    "applyUrl": null,
    "companyName": "Acme Corp"
  },
  {
    "jobId": "job-003",
    "jobTitle": "",
    "city": "Nowhere",
    "country": null,
    "jobCategory": null,
    "datePosted": null,
    "description": null,
    "applyUrl": null,
    "companyName": null
  }
]`

func TestScraper_SiteName(t *testing.T) {
	s := New(nil)
	if got := s.SiteName(); got != model.SiteISmartRecruit {
		t.Errorf("SiteName() = %q, want %q", got, model.SiteISmartRecruit)
	}
}

func TestScraper_Scrape(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(testJSON))
	}))
	defer ts.Close()

	t.Setenv("SCRAPPY_ISMARTRECRUIT_SEEDS", "acme")
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
	if j0.CompanyName != "Acme Corp" {
		t.Errorf("job[0].CompanyName = %q, want %q", j0.CompanyName, "Acme Corp")
	}
	if j0.Location.City != "San Francisco" {
		t.Errorf("job[0].Location.City = %q", j0.Location.City)
	}
	if j0.Location.Country != "USA" {
		t.Errorf("job[0].Location.Country = %q", j0.Location.Country)
	}
	if j0.Description != "Build scalable APIs with Go." {
		t.Errorf("job[0].Description = %q", j0.Description)
	}
	if j0.Site != string(model.SiteISmartRecruit) {
		t.Errorf("job[0].Site = %q", j0.Site)
	}
	if j0.Department != "Engineering" {
		t.Errorf("job[0].Department = %q", j0.Department)
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

	t.Setenv("SCRAPPY_ISMARTRECRUIT_SEEDS", "acme")
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
