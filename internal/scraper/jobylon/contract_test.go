package jobylon

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/arinbalyan/scrappy/internal/model"
)

const testJSON = `[
  {
    "id": 201,
    "title": "Backend Engineer",
    "slug": "backend-engineer-2026",
    "description": "<p>Write <b>Go</b> services.</p>",
    "company": {
      "name": "Acme Corp"
    },
    "locations": [
      {
        "city": "Berlin",
        "country": "Germany"
      }
    ],
    "urls": {
      "ad": "https://jobs.jobylon.com/jobs/backend-engineer-2026/",
      "apply": "https://jobs.jobylon.com/apply/201"
    },
    "from_date": "2026-05-20T08:00:00Z",
    "employment_type": "Full-Time",
    "workspace_type": "on-site",
    "skills": [
      {"label": "Go"},
      {"label": "Kubernetes"}
    ],
    "department": "Engineering"
  },
  {
    "id": 202,
    "title": "Remote Frontend Developer",
    "slug": "remote-frontend-dev",
    "description": null,
    "company": null,
    "locations": [
      {
        "city": "Remote",
        "country": null
      }
    ],
    "urls": null,
    "from_date": "2026-05-18",
    "employment_type": "Contract",
    "workspace_type": "remote",
    "skills": [],
    "department": null
  },
  {
    "id": 203,
    "title": "",
    "slug": "empty-job",
    "description": null,
    "company": null,
    "locations": null,
    "urls": null,
    "from_date": null,
    "employment_type": null,
    "workspace_type": null,
    "skills": null,
    "department": null
  }
]`

func TestScraper_SiteName(t *testing.T) {
	s := New(nil)
	if got := s.SiteName(); got != model.SiteJobylon {
		t.Errorf("SiteName() = %q, want %q", got, model.SiteJobylon)
	}
}

func TestScraper_Scrape(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(testJSON))
	}))
	defer ts.Close()

	t.Setenv("SCRAPPY_JOBYLON_SEEDS", "acme")
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
	if j0.Title != "Backend Engineer" {
		t.Errorf("job[0].Title = %q, want %q", j0.Title, "Backend Engineer")
	}
	if j0.CompanyName != "Acme Corp" {
		t.Errorf("job[0].CompanyName = %q, want %q", j0.CompanyName, "Acme Corp")
	}
	if j0.Location.City != "Berlin" {
		t.Errorf("job[0].Location.City = %q", j0.Location.City)
	}
	if j0.Location.Country != "Germany" {
		t.Errorf("job[0].Location.Country = %q", j0.Location.Country)
	}
	if j0.Description != "Write Go services." {
		t.Errorf("job[0].Description = %q", j0.Description)
	}
	if j0.Site != string(model.SiteJobylon) {
		t.Errorf("job[0].Site = %q", j0.Site)
	}
	if j0.Department != "Engineering" {
		t.Errorf("job[0].Department = %q", j0.Department)
	}
	if j0.JobType != "Full-Time" {
		t.Errorf("job[0].JobType = %q", j0.JobType)
	}
	if len(j0.Skills) != 2 {
		t.Errorf("expected 2 skills, got %d", len(j0.Skills))
	}
	if j0.IsRemote {
		t.Error("job[0].IsRemote should be false")
	}

	// Check second job (remote)
	j1 := jobs[1]
	if j1.Title != "Remote Frontend Developer" {
		t.Errorf("job[1].Title = %q", j1.Title)
	}
	if !j1.IsRemote {
		t.Error("job[1].IsRemote should be true")
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

func TestScraper_Scrape_SearchTerm(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(testJSON))
	}))
	defer ts.Close()

	t.Setenv("SCRAPPY_JOBYLON_SEEDS", "acme")
	s := NewWithAPIURL(nil, ts.URL)
	jobs, err := s.Scrape(context.Background(), model.ScraperInput{
		SearchTerm:    "frontend",
		ResultsWanted: 25,
	})
	if err != nil {
		t.Fatalf("Scrape() returned error: %v", err)
	}

	if len(jobs) != 1 {
		t.Fatalf("expected 1 job matching 'frontend', got %d", len(jobs))
	}
	if jobs[0].Title != "Remote Frontend Developer" {
		t.Errorf("job[0].Title = %q", jobs[0].Title)
	}
}
