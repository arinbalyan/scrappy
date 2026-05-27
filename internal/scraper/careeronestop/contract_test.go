package careeronestop

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/arinbalyan/scrappy/internal/model"
)

const testResponse = `{
  "Jobs": [
    {
      "JvId": "12345",
      "Title": "Software Engineer",
      "Company": "Tech Corp",
      "URL": "https://www.careeronestop.org/job/12345",
      "Location": "San Francisco, CA",
      "Description": "Develop and maintain software applications.",
      "DatePosted": "2026-05-15"
    },
    {
      "JvId": "67890",
      "Title": "Data Analyst",
      "Company": "Data Insights Inc",
      "URL": "https://www.careeronestop.org/job/67890",
      "Location": "New York, NY",
      "Description": "Analyze business data and create reports.",
      "DatePosted": "2026-05-16"
    },
    {
      "JvId": "11111",
      "Title": "Remote DevOps Engineer",
      "Company": "Cloud Native Co",
      "URL": "https://www.careeronestop.org/job/11111",
      "Location": "Remote",
      "Description": "Manage cloud infrastructure.",
      "DatePosted": "2026-05-17"
    }
  ],
  "RecordCount": 3
}`

const emptyResponse = `{"Jobs": [], "RecordCount": 0}`

func TestScraper_SiteName(t *testing.T) {
	s := New(nil)
	if got := s.SiteName(); got != model.SiteCareerOneStop {
		t.Errorf("SiteName() = %q, want %q", got, model.SiteCareerOneStop)
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

	if len(jobs) != 3 {
		t.Fatalf("expected 3 jobs, got %d", len(jobs))
	}

	// Check first job
	j0 := jobs[0]
	if j0.ID != "careeronestop-12345" {
		t.Errorf("job[0].ID = %q, want %q", j0.ID, "careeronestop-12345")
	}
	if j0.Title != "Software Engineer" {
		t.Errorf("job[0].Title = %q, want %q", j0.Title, "Software Engineer")
	}
	if j0.CompanyName != "Tech Corp" {
		t.Errorf("job[0].CompanyName = %q, want %q", j0.CompanyName, "Tech Corp")
	}
	if j0.Location.City != "San Francisco" {
		t.Errorf("job[0].Location.City = %q, want %q", j0.Location.City, "San Francisco")
	}
	if j0.Location.State != "CA" {
		t.Errorf("job[0].Location.State = %q, want %q", j0.Location.State, "CA")
	}
	if j0.Site != string(model.SiteCareerOneStop) {
		t.Errorf("job[0].Site = %q, want %q", j0.Site, model.SiteCareerOneStop)
	}
	if j0.IsRemote {
		t.Errorf("job[0].IsRemote should be false")
	}
	if j0.DatePosted == nil {
		t.Error("job[0].DatePosted is nil")
	}

	// Check remote job
	j2 := jobs[2]
	if j2.Title != "Remote DevOps Engineer" {
		t.Errorf("job[2].Title = %q", j2.Title)
	}
	if !j2.IsRemote {
		t.Errorf("job[2].IsRemote should be true")
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
