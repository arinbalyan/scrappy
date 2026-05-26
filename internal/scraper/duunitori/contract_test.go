package duunitori

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/arinbalyan/scrappy/internal/model"
)

const testResponse = `{
  "results": [
    {
      "slug": "senior-software-engineer-helsinki",
      "heading": "Senior Software Engineer",
      "company_name": "Tech Corp Finland",
      "municipality_name": "Helsinki",
      "descr": "Develop and maintain software applications.",
      "date_posted": "2026-05-15"
    },
    {
      "slug": "data-scientist-espoo",
      "heading": "Data Scientist",
      "company_name": "AI Research Oy",
      "municipality_name": "Espoo",
      "descr": "Analyze business data and create ML models.",
      "date_posted": "2026-05-16"
    }
  ],
  "count": 2
}`

const emptyResponse = `{"results": [], "count": 0}`

func TestScraper_SiteName(t *testing.T) {
	s := New(nil)
	if got := s.SiteName(); got != model.SiteDuunitori {
		t.Errorf("SiteName() = %q, want %q", got, model.SiteDuunitori)
	}
}

func TestScraper_Scrape(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(testResponse))
	}))
	defer ts.Close()

	s := NewWithAPIURL(nil, ts.URL)
	jobs, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 25})
	if err != nil {
		t.Fatalf("Scrape() returned error: %v", err)
	}

	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(jobs))
	}

	// Check first job
	j0 := jobs[0]
	if j0.ID != "duunitori-senior-software-engineer-helsinki" {
		t.Errorf("job[0].ID = %q, want %q", j0.ID, "duunitori-senior-software-engineer-helsinki")
	}
	if j0.Title != "Senior Software Engineer" {
		t.Errorf("job[0].Title = %q, want %q", j0.Title, "Senior Software Engineer")
	}
	if j0.CompanyName != "Tech Corp Finland" {
		t.Errorf("job[0].CompanyName = %q, want %q", j0.CompanyName, "Tech Corp Finland")
	}
	if j0.Location.City != "Helsinki" {
		t.Errorf("job[0].Location.City = %q, want %q", j0.Location.City, "Helsinki")
	}
	if j0.Location.Country != "Finland" {
		t.Errorf("job[0].Location.Country = %q, want %q", j0.Location.Country, "Finland")
	}
	if j0.Site != string(model.SiteDuunitori) {
		t.Errorf("job[0].Site = %q", j0.Site)
	}
	if j0.JobURL != "https://duunitori.fi/tyopaikat/senior-software-engineer-helsinki" {
		t.Errorf("job[0].JobURL = %q", j0.JobURL)
	}
	if j0.DatePosted == nil {
		t.Error("job[0].DatePosted is nil")
	}

	// Check second job
	j1 := jobs[1]
	if j1.ID != "duunitori-data-scientist-espoo" {
		t.Errorf("job[1].ID = %q", j1.ID)
	}
	if j1.Title != "Data Scientist" {
		t.Errorf("job[1].Title = %q", j1.Title)
	}
	if j1.Location.City != "Espoo" {
		t.Errorf("job[1].Location.City = %q", j1.Location.City)
	}
}

func TestScraper_Scrape_EmptyBody(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(emptyResponse))
	}))
	defer ts.Close()

	s := NewWithAPIURL(nil, ts.URL)
	_, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 25})
	if err == nil {
		t.Fatal("expected error for empty results, got nil")
	}
}

func TestScraper_Scrape_HTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	s := NewWithAPIURL(nil, ts.URL)
	_, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 25})
	if err == nil {
		t.Fatal("expected error for 500 status, got nil")
	}
}
