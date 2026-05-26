package lever

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/arinbalyan/scrappy/internal/model"
)

const testJSON = `[
  {
    "id": "abc-123",
    "text": "Senior Software Engineer",
    "descriptionPlain": "Build and maintain our core platform.",
    "description": "<p>Build and maintain our <b>core platform</b>.</p>",
    "descriptionBody": "<p>You will work on the backend services.</p>",
    "descriptionBodyPlain": "You will work on the backend services.",
    "additional": null,
    "additionalPlain": null,
    "categories": {
      "location": "San Francisco, CA",
      "team": "Engineering",
      "commitment": "Full-Time",
      "allLocations": ["San Francisco, CA"]
    },
    "createdAt": 1747728000000,
    "workplaceType": "on-site",
    "hostedUrl": "https://jobs.lever.co/acme/abc-123",
    "applyUrl": "https://jobs.lever.co/acme/abc-123/apply",
    "lists": []
  },
  {
    "id": "def-456",
    "text": "Remote UX Designer",
    "descriptionPlain": null,
    "description": null,
    "descriptionBody": null,
    "descriptionBodyPlain": null,
    "additional": null,
    "additionalPlain": null,
    "categories": {
      "location": "Remote",
      "team": "Design",
      "commitment": "Contract",
      "allLocations": ["Remote"]
    },
    "createdAt": 1747641600000,
    "workplaceType": "remote",
    "hostedUrl": null,
    "applyUrl": null,
    "lists": []
  },
  {
    "id": "ghi-789",
    "text": "",
    "categories": null,
    "createdAt": null,
    "workplaceType": null,
    "hostedUrl": null,
    "applyUrl": null,
    "lists": []
  }
]`

func TestScraper_SiteName(t *testing.T) {
	s := New(nil)
	if got := s.SiteName(); got != model.SiteLever {
		t.Errorf("SiteName() = %q, want %q", got, model.SiteLever)
	}
}

func TestScraper_Scrape(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(testJSON))
	}))
	defer ts.Close()

	t.Setenv("SCRAPPY_LEVER_SEEDS", "acme")
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
	if j0.Location.City != "San Francisco, CA" {
		t.Errorf("job[0].Location.City = %q", j0.Location.City)
	}
	if j0.Description != "Build and maintain our core platform." {
		t.Errorf("job[0].Description = %q", j0.Description)
	}
	if j0.Site != string(model.SiteLever) {
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
	if j0.DatePosted == nil {
		t.Error("job[0].DatePosted should not be nil")
	}

	// Check second job (remote)
	j1 := jobs[1]
	if j1.Title != "Remote UX Designer" {
		t.Errorf("job[1].Title = %q", j1.Title)
	}
	if !j1.IsRemote {
		t.Error("job[1].IsRemote should be true")
	}
	if j1.JobURL == "" {
		t.Error("job[1].JobURL should have fallback URL")
	}
	if j1.Department != "Design" {
		t.Errorf("job[1].Department = %q", j1.Department)
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
