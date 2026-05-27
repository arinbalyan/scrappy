package icims

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
      "id": "12345",
      "title": "Software Engineer",
      "url": "/jobs/12345/job",
      "location": "San Francisco, CA",
      "datePosted": "2026-05-20",
      "category": "Engineering"
    },
    {
      "id": "67890",
      "title": "Remote Full Stack Developer",
      "url": "/jobs/67890/job",
      "location": "Remote",
      "datePosted": "2026-05-18",
      "category": "Engineering"
    },
    {
      "id": "99999",
      "title": "",
      "url": null,
      "location": null,
      "datePosted": null,
      "category": null
    }
  ],
  "totalCount": 2
}`

func TestScraper_SiteName(t *testing.T) {
	s := New(nil)
	if got := s.SiteName(); got != model.SiteICIMS {
		t.Errorf("SiteName() = %q, want %q", got, model.SiteICIMS)
	}
}

func TestScraper_Scrape(t *testing.T) {
	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(testJSON))
	}))
	defer ts.Close()

	t.Setenv("SCRAPPY_ICIMS_SEEDS", "acme")
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
	if j0.Location.City != "San Francisco, CA" {
		t.Errorf("job[0].Location.City = %q", j0.Location.City)
	}
	if j0.IsRemote {
		t.Error("job[0].IsRemote should be false")
	}
	if j0.Site != string(model.SiteICIMS) {
		t.Errorf("job[0].Site = %q", j0.Site)
	}
	if j0.Department != "Engineering" {
		t.Errorf("job[0].Department = %q", j0.Department)
	}
	if j0.DatePosted == nil {
		t.Error("job[0].DatePosted should not be nil")
	}

	// Check second job (remote)
	j1 := jobs[1]
	if j1.Title != "Remote Full Stack Developer" {
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
