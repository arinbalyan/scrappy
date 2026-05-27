package joincom

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/arinbalyan/scrappy/internal/model"
)

const testHTML = `<!DOCTYPE html>
<html>
<head><title>Acme Corp Careers</title></head>
<body>
<script>
window.__INITIAL_STATE__ = {"company":{"id":12345,"name":"Acme Corp"}};
</script>
</body>
</html>`

const testJobsPage = `{
  "items": [
    {
      "id": 5001,
      "title": "Software Engineer",
      "description": "<p>Build cool stuff.</p>",
      "locations": [
        {
          "id": 1,
          "name": "Berlin",
          "city": "Berlin",
          "country": "Germany",
          "isRemote": false
        }
      ],
      "shareableUrl": "https://join.com/jobs/5001",
      "publishedAt": "2026-05-20T08:00:00Z",
      "employmentType": "Full-Time",
      "remoteOption": "on-site",
      "department": {
        "name": "Engineering"
      }
    },
    {
      "id": 5002,
      "title": "Remote Product Manager",
      "description": null,
      "locations": [
        {
          "id": 2,
          "name": "Remote",
          "city": null,
          "country": null,
          "isRemote": true
        }
      ],
      "shareableUrl": null,
      "publishedAt": "2026-05-18",
      "employmentType": "Contract",
      "remoteOption": "remote",
      "department": null
    },
    {
      "id": 5003,
      "title": "",
      "locations": [],
      "shareableUrl": null
    }
  ],
  "pagination": {
    "page": 1,
    "pageSize": 50,
    "total": 2,
    "totalPages": 1
  }
}`

func TestScraper_SiteName(t *testing.T) {
	s := New(nil)
	if got := s.SiteName(); got != model.SiteJoinCom {
		t.Errorf("SiteName() = %q, want %q", got, model.SiteJoinCom)
	}
}

func TestScraper_Scrape(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Step 1: return HTML for HTML pages
		if strings.Contains(r.URL.Path, "/companies/") && !strings.Contains(r.URL.Path, "/jobs") {
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(testHTML))
			return
		}
		// Step 2: return jobs page
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(testJobsPage))
	}))
	defer ts.Close()

	t.Setenv("SCRAPPY_JOINCOM_SEEDS", "acme")
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
	if j0.CompanyName != "Acme" {
		t.Errorf("job[0].CompanyName = %q, want %q", j0.CompanyName, "Acme")
	}
	if j0.Location.City != "Berlin" {
		t.Errorf("job[0].Location.City = %q", j0.Location.City)
	}
	if j0.Location.Country != "Germany" {
		t.Errorf("job[0].Location.Country = %q", j0.Location.Country)
	}
	if j0.IsRemote {
		t.Error("job[0].IsRemote should be false")
	}
	if j0.Site != string(model.SiteJoinCom) {
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
	if j1.Title != "Remote Product Manager" {
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
