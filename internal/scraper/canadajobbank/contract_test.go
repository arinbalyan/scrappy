package canadajobbank

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/arinbalyan/scrappy/internal/model"
)

const testResponse = `{
  "success": true,
  "result": {
    "records": [
      {
        "_id": "100001",
        "Job Title": "Software Engineer",
        "Original Job Title": "Software Engineer",
        "Company": "Tech Corp Canada",
        "City": "Toronto",
        "Province/Territory": "Ontario",
        "Salary Minimum": 80000,
        "Salary Maximum": 120000,
        "Salary Per": "Yearly",
        "First Posting Date": "2026-05-15",
        "NOC21 Code Name": "Software Engineers",
        "Employment Type": "Full-Time",
        "Employment Term": "Permanent",
        "Education LOS": "Bachelor's Degree",
        "Experience Level": "2 years to less than 3 years",
        "Vacancy Count": 2
      },
      {
        "_id": "100002",
        "Job Title": "Data Scientist",
        "Original Job Title": "Data Scientist",
        "Company": "AI Research Inc",
        "City": "Vancouver",
        "Province/Territory": "British Columbia",
        "Salary Minimum": 90000,
        "Salary Maximum": 150000,
        "Salary Per": "Yearly",
        "First Posting Date": "2026-05-16",
        "NOC21 Code Name": "Data Scientists",
        "Employment Type": "Full-Time",
        "Employment Term": "Permanent",
        "Education LOS": "Master's Degree",
        "Experience Level": "3 years to less than 5 years",
        "Vacancy Count": 1
      }
    ],
    "total": 2
  }
}`

const emptyResponse = `{"success": true, "result": {"records": [], "total": 0}}`

const errorResponse = `{"success": false}`

func TestScraper_SiteName(t *testing.T) {
	s := New(nil)
	if got := s.SiteName(); got != model.SiteCanadaJobBank {
		t.Errorf("SiteName() = %q, want %q", got, model.SiteCanadaJobBank)
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
	if j0.ID != "canadajobbank-100001" {
		t.Errorf("job[0].ID = %q, want %q", j0.ID, "canadajobbank-100001")
	}
	if j0.Title != "Software Engineer" {
		t.Errorf("job[0].Title = %q, want %q", j0.Title, "Software Engineer")
	}
	if j0.Location.City != "Toronto" {
		t.Errorf("job[0].Location.City = %q, want %q", j0.Location.City, "Toronto")
	}
	if j0.Location.State != "Ontario" {
		t.Errorf("job[0].Location.State = %q, want %q", j0.Location.State, "Ontario")
	}
	if j0.Location.Country != "Canada" {
		t.Errorf("job[0].Location.Country = %q, want %q", j0.Location.Country, "Canada")
	}
	if j0.Site != string(model.SiteCanadaJobBank) {
		t.Errorf("job[0].Site = %q, want %q", j0.Site, model.SiteCanadaJobBank)
	}
	if j0.JobURL != "https://www.jobbank.gc.ca/jobsearch/jobposting/100001" {
		t.Errorf("job[0].JobURL = %q", j0.JobURL)
	}
	if j0.Compensation == nil {
		t.Fatal("job[0].Compensation is nil")
	}
	if j0.Compensation.Currency != "CAD" {
		t.Errorf("job[0].Compensation.Currency = %q, want %q", j0.Compensation.Currency, "CAD")
	}
	if j0.DatePosted == nil {
		t.Error("job[0].DatePosted is nil")
	}

	// Check second job
	j1 := jobs[1]
	if j1.ID != "canadajobbank-100002" {
		t.Errorf("job[1].ID = %q", j1.ID)
	}
	if j1.Title != "Data Scientist" {
		t.Errorf("job[1].Title = %q", j1.Title)
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

func TestScraper_Scrape_APIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(errorResponse))
	}))
	defer ts.Close()

	s := NewWithAPIURL(nil, ts.URL)
	_, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 25})
	if err == nil {
		t.Fatal("expected error for unsuccessful API response, got nil")
	}
}
