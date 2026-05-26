package reliefweb

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/arinbalyan/scrappy/internal/model"
)

const testAPIResponse = `{
  "data": [
    {
      "id": "12345",
      "score": 0.95,
      "href": "https://reliefweb.int/job/12345",
      "fields": {
        "title": "Humanitarian Coordinator",
        "body": "<p>Coordinate humanitarian response in the region.</p>",
        "url": "https://reliefweb.int/job/12345/humanitarian-coordinator",
        "source": [{"name": "UN OCHA"}],
        "date": {"created": "2026-05-15T10:00:00Z"},
        "country": [{"name": "South Sudan", "iso3": "SSD"}],
        "theme": [{"name": "Humanitarian"}],
        "type": [{"name": "Job"}]}
    },
    {
      "id": "67890",
      "score": 0.88,
      "href": "https://reliefweb.int/job/67890",
      "fields": {
        "title": "Logistics Officer",
        "body": "<p>Manage supply chain logistics.</p>",
        "url": "",
        "source": [{"name": "WFP"}],
        "date": {"created": "2026-05-14T08:30:00Z"},
        "country": [
          {"name": "Kenya", "iso3": "KEN"},
          {"name": "Ethiopia", "iso3": "ETH"}
        ],
        "theme": [{"name": "Logistics"}],
        "type": [{"name": "Job"}]}
    },
    {
      "id": "empty-title",
      "score": 0.0,
      "href": "https://reliefweb.int/job/empty-title",
      "fields": {
        "title": "",
        "body": "",
        "url": "",
        "source": [],
        "date": {},
        "country": [],
        "theme": [],
        "type": []
      }
    }
  ]
}`

func TestScraper_SiteName(t *testing.T) {
	s := New(nil)
	if got := s.SiteName(); got != model.SiteReliefWeb {
		t.Errorf("SiteName() = %q, want %q", got, model.SiteReliefWeb)
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
	jobs, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 25})
	if err != nil {
		t.Fatalf("Scrape() returned error: %v", err)
	}

	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(jobs))
	}

	j1 := jobs[0]
	if j1.ID != "reliefweb-12345" {
		t.Errorf("job[0].ID = %q, want %q", j1.ID, "reliefweb-12345")
	}
	if j1.Title != "Humanitarian Coordinator" {
		t.Errorf("job[0].Title = %q, want %q", j1.Title, "Humanitarian Coordinator")
	}
	if j1.CompanyName != "UN OCHA" {
		t.Errorf("job[0].CompanyName = %q, want %q", j1.CompanyName, "UN OCHA")
	}
	if j1.JobURL != "https://reliefweb.int/job/12345/humanitarian-coordinator" {
		t.Errorf("job[0].JobURL = %q", j1.JobURL)
	}
	if j1.Site != string(model.SiteReliefWeb) {
		t.Errorf("job[0].Site = %q, want %q", j1.Site, model.SiteReliefWeb)
	}
	if j1.Location.Country != "South Sudan" {
		t.Errorf("job[0].Location.Country = %q, want %q", j1.Location.Country, "South Sudan")
	}
	if j1.DatePosted == nil {
		t.Error("job[0].DatePosted is nil, expected a parsed date")
	}

	j2 := jobs[1]
	if j2.ID != "reliefweb-67890" {
		t.Errorf("job[1].ID = %q, want %q", j2.ID, "reliefweb-67890")
	}
	if j2.Title != "Logistics Officer" {
		t.Errorf("job[1].Title = %q, want %q", j2.Title, "Logistics Officer")
	}
	if j2.CompanyName != "WFP" {
		t.Errorf("job[1].CompanyName = %q, want %q", j2.CompanyName, "WFP")
	}
	// Should fall back to href when url is empty
	if !strings.Contains(j2.JobURL, "reliefweb.int/job/67890") {
		t.Errorf("job[1].JobURL = %q, should contain href fallback", j2.JobURL)
	}
	// Multiple countries: first is country, comma-joined is city
	if j2.Location.Country != "Kenya" {
		t.Errorf("job[1].Location.Country = %q, want %q", j2.Location.Country, "Kenya")
	}
	if !strings.Contains(j2.Location.City, "Ethiopia") {
		t.Errorf("job[1].Location.City = %q, should contain 'Ethiopia'", j2.Location.City)
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

func TestScraper_Scrape_EmptyBody(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data":[]}`))
	}))
	defer ts.Close()

	s := NewWithAPIURL(nil, ts.URL)
	_, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 25})
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
