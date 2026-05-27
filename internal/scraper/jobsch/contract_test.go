package jobsch

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
)

const testJSON = `{
  "documents": [
    {
      "job_id": "12345",
      "title": "Senior Software Engineer",
      "company_name": "TechCorp Zurich",
      "preview": "Build scalable backend systems in Go.",
      "publication_date": "2026-05-15T10:00:00Z",
      "_links": {
        "detail_en": { "href": "https://www.jobs.ch/en/vacancies/detail/12345/" }
      }
    },
    {
      "job_id": "67890",
      "title": "Data Scientist",
      "company_name": "AI Labs Basel",
      "preview": "<p>Design and implement ML models.</p>",
      "publication_date": "2026-05-16T14:30:00Z",
      "_links": {
        "detail_en": { "href": "" }
      }
    },
    {
      "job_id": "99999",
      "title": "",
      "company_name": "Empty Inc",
      "preview": "Empty title job",
      "publication_date": "2026-05-17T08:00:00Z",
      "_links": {
        "detail_en": { "href": "" }
      }
    }
  ],
  "num_pages": 1
}`

func TestScraper_SiteName(t *testing.T) {
	s := New(nil)
	if got := s.SiteName(); got != model.SiteJobsCH {
		t.Errorf("SiteName() = %q, want %q", got, model.SiteJobsCH)
	}
}

func TestScraper_Scrape(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("rows") != "25" {
			t.Errorf("expected rows=25, got %q", r.URL.Query().Get("rows"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(testJSON))
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
	if j0.ID != "jobsch-12345" {
		t.Errorf("job[0].ID = %q, want %q", j0.ID, "jobsch-12345")
	}
	if j0.Title != "Senior Software Engineer" {
		t.Errorf("job[0].Title = %q", j0.Title)
	}
	if j0.CompanyName != "TechCorp Zurich" {
		t.Errorf("job[0].CompanyName = %q", j0.CompanyName)
	}
	if j0.JobURL != "https://www.jobs.ch/en/vacancies/detail/12345/" {
		t.Errorf("job[0].JobURL = %q", j0.JobURL)
	}
	if j0.Site != string(model.SiteJobsCH) {
		t.Errorf("job[0].Site = %q", j0.Site)
	}
	if j0.Location.Country != "Switzerland" {
		t.Errorf("job[0].Location.Country = %q", j0.Location.Country)
	}
	if j0.DatePosted == nil {
		t.Error("job[0].DatePosted is nil")
	} else if !j0.DatePosted.Before(time.Now()) {
		t.Error("job[0].DatePosted should be in the past")
	}

	// Check second job - empty _links.detail_en.href should use fallback URL
	j1 := jobs[1]
	if j1.ID != "jobsch-67890" {
		t.Errorf("job[1].ID = %q", j1.ID)
	}
	if j1.CompanyName != "AI Labs Basel" {
		t.Errorf("job[1].CompanyName = %q", j1.CompanyName)
	}
	expectedURL := "https://www.jobs.ch/en/vacancies/detail/67890/"
	if j1.JobURL != expectedURL {
		t.Errorf("job[1].JobURL = %q, want %q", j1.JobURL, expectedURL)
	}
}

func TestScraper_Scrape_SearchTerm(t *testing.T) {
	var gotQuery string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query().Get("query")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(testJSON))
	}))
	defer ts.Close()

	s := NewWithAPIURL(nil, ts.URL)
	_, err := s.Scrape(context.Background(), model.ScraperInput{
		SearchTerm:    "engineer",
		ResultsWanted: 25,
	})
	if err != nil {
		t.Fatalf("Scrape() returned error: %v", err)
	}
	if gotQuery != "engineer" {
		t.Errorf("query param = %q, want %q", gotQuery, "engineer")
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
		w.Write([]byte(`{"documents":[],"num_pages":0}`))
	}))
	defer ts.Close()

	s := NewWithAPIURL(nil, ts.URL)
	_, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 25})
	if err == nil {
		t.Fatal("expected error for empty documents, got nil")
	}
}

func TestNewWithAPIURL_EmptyEndpoint(t *testing.T) {
	s1 := NewWithAPIURL(nil, "")
	s2 := New(nil)
	if s1.apiURL != s2.apiURL {
		t.Errorf("empty endpoint should not override API URL")
	}
}
