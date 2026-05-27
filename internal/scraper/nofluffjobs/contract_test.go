package nofluffjobs

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/arinbalyan/scrappy/internal/model"
)

const testAPIResponse = `[
  {
    "id": "abc123",
    "name": "TechCorp",
    "title": "Senior Go Developer",
    "technology": "Go",
    "category": "Engineering",
    "seniority": ["senior", "mid"],
    "location": {
      "places": [
        {
          "country": { "code": "PL", "name": "Poland" },
          "city": "Warsaw"
        }
      ],
      "fullyRemote": false
    },
    "salary": {
      "from": 200000,
      "to": 350000,
      "currency": "PLN",
      "type": "B2B"
    },
    "posted": 1715760000000,
    "url": "senior-go-developer-techcorp",
    "regions": ["Warszawa", "Remote"]
  },
  {
    "id": "def456",
    "name": "StartupInc",
    "title": "Full Stack Engineer",
    "technology": "JavaScript",
    "category": "Engineering",
    "seniority": ["mid"],
    "location": {
      "places": [
        {
          "country": { "code": "PL", "name": "Poland" },
          "city": "Krakow"
        }
      ],
      "fullyRemote": true
    },
    "salary": {
      "from": 150000,
      "to": 250000,
      "currency": "PLN",
      "type": "B2B"
    },
    "posted": 1715673600000,
    "url": "full-stack-engineer-startupinc",
    "regions": ["Krakow", "Remote"]
  },
  {
    "id": "empty-title",
    "name": "NoName",
    "title": "",
    "technology": "Python",
    "category": "",
    "seniority": [],
    "location": {
      "places": [],
      "fullyRemote": false
    },
    "salary": {
      "from": 0,
      "to": 0,
      "currency": "",
      "type": ""
    },
    "posted": 0,
    "url": "",
    "regions": []
  }
]`

func TestScraper_SiteName(t *testing.T) {
	s := New(nil)
	if got := s.SiteName(); got != model.SiteNoFluffJobs {
		t.Errorf("SiteName() = %q, want %q", got, model.SiteNoFluffJobs)
	}
}

func TestScraper_Scrape(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(testAPIResponse))
	}))
	defer ts.Close()

	s := NewWithAPIURL(nil, ts.URL)
	jobs, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 10})
	if err != nil {
		t.Fatalf("Scrape() returned error: %v", err)
	}

	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(jobs))
	}

	// Check first job
	j1 := jobs[0]
	if j1.ID != "nofluffjobs-abc123" {
		t.Errorf("job[0].ID = %q, want %q", j1.ID, "nofluffjobs-abc123")
	}
	if j1.Title != "Senior Go Developer" {
		t.Errorf("job[0].Title = %q, want %q", j1.Title, "Senior Go Developer")
	}
	if j1.CompanyName != "TechCorp" {
		t.Errorf("job[0].CompanyName = %q, want %q", j1.CompanyName, "TechCorp")
	}
	if j1.JobURL != "https://nofluffjobs.com/job/senior-go-developer-techcorp" {
		t.Errorf("job[0].JobURL = %q", j1.JobURL)
	}
	if j1.Site != string(model.SiteNoFluffJobs) {
		t.Errorf("job[0].Site = %q, want %q", j1.Site, model.SiteNoFluffJobs)
	}
	if j1.IsRemote != false {
		t.Error("job[0].IsRemote should be false")
	}
	if j1.Location.City != "Warsaw" {
		t.Errorf("job[0].Location.City = %q, want %q", j1.Location.City, "Warsaw")
	}
	if j1.Location.Country != "Poland" {
		t.Errorf("job[0].Location.Country = %q, want %q", j1.Location.Country, "Poland")
	}
	if j1.Compensation == nil {
		t.Fatal("job[0].Compensation is nil, expected a Compensation")
	}
	if j1.Compensation.Currency != "PLN" {
		t.Errorf("job[0].Compensation.Currency = %q, want %q", j1.Compensation.Currency, "PLN")
	}
	if j1.Compensation.MinAmount == nil || *j1.Compensation.MinAmount != 200000 {
		t.Errorf("job[0].Compensation.MinAmount = %v, want 200000", j1.Compensation.MinAmount)
	}
	if j1.DatePosted == nil {
		t.Error("job[0].DatePosted is nil, expected a parsed date")
	}
	if j1.Description == "" {
		t.Error("job[0].Description is empty, expected built description")
	}

	// Check second job
	j2 := jobs[1]
	if j2.ID != "nofluffjobs-def456" {
		t.Errorf("job[1].ID = %q, want %q", j2.ID, "nofluffjobs-def456")
	}
	if j2.Title != "Full Stack Engineer" {
		t.Errorf("job[1].Title = %q, want %q", j2.Title, "Full Stack Engineer")
	}
	if j2.IsRemote != true {
		t.Error("job[1].IsRemote should be true")
	}
	if j2.CompanyName != "StartupInc" {
		t.Errorf("job[1].CompanyName = %q, want %q", j2.CompanyName, "StartupInc")
	}
}

func TestScraper_Scrape_SearchTerm(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(testAPIResponse))
	}))
	defer ts.Close()

	s := NewWithAPIURL(nil, ts.URL)
	jobs, err := s.Scrape(context.Background(), model.ScraperInput{
		SearchTerm:    "go",
		ResultsWanted: 10,
	})
	if err != nil {
		t.Fatalf("Scrape() returned error: %v", err)
	}

	if len(jobs) != 1 {
		t.Fatalf("expected 1 job matching 'go', got %d", len(jobs))
	}
	if jobs[0].Title != "Senior Go Developer" {
		t.Errorf("job[0].Title = %q, want %q", jobs[0].Title, "Senior Go Developer")
	}
}

func TestScraper_Scrape_HTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	s := NewWithAPIURL(nil, ts.URL)
	_, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 10})
	if err == nil {
		t.Fatal("expected error for 500 status, got nil")
	}
}

func TestScraper_Scrape_EmptyBody(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[]`))
	}))
	defer ts.Close()

	s := NewWithAPIURL(nil, ts.URL)
	_, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 10})
	if err == nil {
		t.Fatal("expected error for empty response, got nil")
	}
}

func TestNewWithAPIURL_EmptyEndpoint(t *testing.T) {
	s1 := NewWithAPIURL(nil, "")
	s2 := New(nil)
	if s1.apiURL != s2.apiURL {
		t.Errorf("empty endpoint should not override API URL")
	}
}
