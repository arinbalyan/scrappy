package jobdataapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/arinbalyan/scrappy/internal/model"
)

const testResponse = `{
  "count": 3,
  "next": null,
  "previous": null,
  "results": [
    {
      "id": 12345,
      "title": "Software Engineer",
      "slug": "software-engineer-acme-corp",
      "company": { "name": "Acme Corp", "logo": null },
      "has_remote": true,
      "location": { "city": "San Francisco", "country": "US" },
      "description": "Build cool stuff with Go and React.",
      "application_url": "https://acme.com/apply/12345",
      "job_type": "full-time",
      "salary_min": 100000,
      "salary_max": 150000,
      "salary_currency": "USD",
      "date_posted": "2026-05-15T10:00:00Z",
      "tags": ["Go", "React", "Kubernetes"]
    },
    {
      "id": 67890,
      "title": "Data Scientist",
      "slug": "data-scientist-data-inc",
      "company": { "name": "Data Inc", "logo": null },
      "has_remote": false,
      "location": { "city": "New York", "country": "US" },
      "description": "Analyze data and build ML models.",
      "application_url": "",
      "job_type": "full-time",
      "salary_min": null,
      "salary_max": 200000,
      "salary_currency": "USD",
      "date_posted": "2026-05-16T14:30:00Z",
      "tags": ["Python", "ML"]
    },
    {
      "id": 11111,
      "title": "",
      "slug": "empty-title-job",
      "company": { "name": "Ghost Inc" },
      "has_remote": false,
      "description": "Empty title",
      "date_posted": "2026-05-17T08:00:00Z",
      "tags": []
    }
  ]
}`

func TestScraper_SiteName(t *testing.T) {
	s := New(nil)
	if got := s.SiteName(); got != model.SiteJobDataAPI {
		t.Errorf("SiteName() = %q, want %q", got, model.SiteJobDataAPI)
	}
}

func TestScraper_Scrape(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify query params
		if r.URL.Query().Get("page") != "1" {
			t.Errorf("expected page=1, got %s", r.URL.Query().Get("page"))
		}
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
	j1 := jobs[0]
	if j1.ID != "jobdataapi-12345" {
		t.Errorf("job[0].ID = %q, want %q", j1.ID, "jobdataapi-12345")
	}
	if j1.Title != "Software Engineer" {
		t.Errorf("job[0].Title = %q, want %q", j1.Title, "Software Engineer")
	}
	if j1.CompanyName != "Acme Corp" {
		t.Errorf("job[0].CompanyName = %q, want %q", j1.CompanyName, "Acme Corp")
	}
	if j1.JobURL != "https://acme.com/apply/12345" {
		t.Errorf("job[0].JobURL = %q", j1.JobURL)
	}
	if !j1.IsRemote {
		t.Error("job[0].IsRemote should be true")
	}
	if j1.Location.City != "San Francisco" {
		t.Errorf("job[0].Location.City = %q, want %q", j1.Location.City, "San Francisco")
	}
	if j1.Location.Country != "US" {
		t.Errorf("job[0].Location.Country = %q, want %q", j1.Location.Country, "US")
	}
	if j1.Site != string(model.SiteJobDataAPI) {
		t.Errorf("job[0].Site = %q, want %q", j1.Site, model.SiteJobDataAPI)
	}
	if j1.Compensation == nil {
		t.Fatal("job[0].Compensation should not be nil")
	}
	if j1.Compensation.Interval != model.IntervalYearly {
		t.Errorf("job[0].Compensation.Interval = %q, want %q", j1.Compensation.Interval, model.IntervalYearly)
	}
	if j1.Compensation.Currency != "USD" {
		t.Errorf("job[0].Compensation.Currency = %q, want %q", j1.Compensation.Currency, "USD")
	}
	if *j1.Compensation.MinAmount != 100000 {
		t.Errorf("job[0].Compensation.MinAmount = %v, want 100000", *j1.Compensation.MinAmount)
	}
	if *j1.Compensation.MaxAmount != 150000 {
		t.Errorf("job[0].Compensation.MaxAmount = %v, want 150000", *j1.Compensation.MaxAmount)
	}
	if len(j1.Skills) != 3 || j1.Skills[0] != "Go" {
		t.Errorf("job[0].Skills = %v, want [Go React Kubernetes]", j1.Skills)
	}
	if j1.ApplyMethod != "external_url" {
		t.Errorf("job[0].ApplyMethod = %q, want %q", j1.ApplyMethod, "external_url")
	}

	// Check second job
	j2 := jobs[1]
	if j2.ID != "jobdataapi-67890" {
		t.Errorf("job[1].ID = %q, want %q", j2.ID, "jobdataapi-67890")
	}
	if j2.Title != "Data Scientist" {
		t.Errorf("job[1].Title = %q, want %q", j2.Title, "Data Scientist")
	}
	if j2.JobURL != "https://jobdataapi.com/jobs/data-scientist-data-inc/" {
		t.Errorf("job[1].JobURL = %q (expected fallback URL)", j2.JobURL)
	}
	if j2.IsRemote {
		t.Error("job[1].IsRemote should be false")
	}
	if j2.Compensation == nil {
		t.Fatal("job[1].Compensation should not be nil")
	}
	if j2.Compensation.MinAmount != nil {
		t.Error("job[1].Compensation.MinAmount should be nil (only max provided)")
	}
	if *j2.Compensation.MaxAmount != 200000 {
		t.Errorf("job[1].Compensation.MaxAmount = %v, want 200000", *j2.Compensation.MaxAmount)
	}
	if len(j2.Skills) != 2 || j2.Skills[0] != "Python" {
		t.Errorf("job[1].Skills = %v, want [Python ML]", j2.Skills)
	}
}

func TestScraper_Scrape_SearchTerm(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("title") != "engineer" {
			t.Errorf("expected title=engineer, got %s", r.URL.Query().Get("title"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(testResponse))
	}))
	defer ts.Close()

	s := NewWithAPIURL(nil, ts.URL)
	jobs, err := s.Scrape(context.Background(), model.ScraperInput{
		SearchTerm:    "engineer",
		ResultsWanted: 25,
	})
	if err != nil {
		t.Fatalf("Scrape() returned error: %v", err)
	}

	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(jobs))
	}
}

func TestScraper_Scrape_Location(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		loc := r.URL.Query().Get("location_search")
		if loc != "San Francisco" {
			t.Errorf("expected location_search=San Francisco, got %s", loc)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(testResponse))
	}))
	defer ts.Close()

	s := NewWithAPIURL(nil, ts.URL)
	jobs, err := s.Scrape(context.Background(), model.ScraperInput{
		Location:      "San Francisco",
		ResultsWanted: 25,
	})
	if err != nil {
		t.Fatalf("Scrape() returned error: %v", err)
	}

	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(jobs))
	}
}

func TestScraper_FailsOnEmptyResponse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"count":0,"next":null,"previous":null,"results":[]}`))
	}))
	defer ts.Close()

	s := NewWithAPIURL(nil, ts.URL)
	_, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 25})
	if err == nil {
		t.Fatal("expected error for empty response, got nil")
	}
}

func TestScraper_FailsOn429And503(t *testing.T) {
	tests := []struct {
		name string
		code int
	}{
		{"429 Too Many Requests", http.StatusTooManyRequests},
		{"503 Service Unavailable", http.StatusServiceUnavailable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.code)
			}))
			defer ts.Close()

			s := NewWithAPIURL(nil, ts.URL)
			_, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 25})
			if err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}
