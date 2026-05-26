package habrcareer

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/arinbalyan/scrappy/internal/model"
)

const testResponse = `{
  "list": [
    {
      "id": 12345,
      "href": "/vacancy/12345/",
      "title": "Senior Go Developer",
      "remoteWork": true,
      "salaryQualification": { "title": "Senior" },
      "publishedDate": { "date": "2026-05-15T10:00:00Z" },
      "company": { "title": "TechCorp", "href": "/company/techcorp/" },
      "employment": "full_time",
      "salary": { "from": 300000, "to": 500000, "currency": "RUR" },
      "divisions": [{ "title": "Backend" }],
      "skills": [{ "title": "Go" }, { "title": "PostgreSQL" }],
      "locations": [{ "title": "Moscow" }]
    },
    {
      "id": 67890,
      "href": "/vacancy/67890/",
      "title": "Go Backend Engineer",
      "remoteWork": false,
      "publishedDate": { "date": "2026-05-16T14:30:00Z" },
      "company": { "title": "Startup Inc" },
      "employment": "part_time",
      "salary": { "to": 200000, "currency": "RUR" },
      "locations": [{ "title": "Saint Petersburg" }]
    },
    {
      "id": 11111,
      "href": null,
      "title": "No URL Job",
      "company": { "title": "Missing" }
    },
    {
      "id": 22222,
      "href": "/vacancy/22222/",
      "title": "",
      "company": { "title": "Empty Title" }
    }
  ],
  "meta": { "totalResults": 4 }
}`

func TestScraper_SiteName(t *testing.T) {
	s := New(nil)
	if got := s.SiteName(); got != model.SiteHabrCareer {
		t.Errorf("SiteName() = %q, want %q", got, model.SiteHabrCareer)
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
	j1 := jobs[0]
	if j1.ID != "habrcareer-12345" {
		t.Errorf("job[0].ID = %q, want %q", j1.ID, "habrcareer-12345")
	}
	if j1.Title != "Senior Go Developer" {
		t.Errorf("job[0].Title = %q, want %q", j1.Title, "Senior Go Developer")
	}
	if j1.JobURL != "https://career.habr.com/vacancy/12345/" {
		t.Errorf("job[0].JobURL = %q", j1.JobURL)
	}
	if j1.Site != string(model.SiteHabrCareer) {
		t.Errorf("job[0].Site = %q, want %q", j1.Site, model.SiteHabrCareer)
	}
	if !j1.IsRemote {
		t.Error("job[0].IsRemote should be true")
	}
	if j1.CompanyName != "TechCorp" {
		t.Errorf("job[0].CompanyName = %q, want %q", j1.CompanyName, "TechCorp")
	}
	if j1.JobType != "fulltime" {
		t.Errorf("job[0].JobType = %q, want %q", j1.JobType, "fulltime")
	}
	if j1.Location.City != "Moscow" {
		t.Errorf("job[0].Location.City = %q, want %q", j1.Location.City, "Moscow")
	}
	if j1.Compensation == nil {
		t.Fatal("job[0].Compensation should not be nil")
	}
	if j1.Compensation.Interval != model.IntervalMonthly {
		t.Errorf("job[0].Compensation.Interval = %q, want %q", j1.Compensation.Interval, model.IntervalMonthly)
	}
	if j1.Compensation.Currency != "RUB" {
		t.Errorf("job[0].Compensation.Currency = %q, want %q", j1.Compensation.Currency, "RUB")
	}
	if j1.Compensation.MinAmount == nil || *j1.Compensation.MinAmount != 300000 {
		t.Errorf("job[0].Compensation.MinAmount = %v, want 300000", j1.Compensation.MinAmount)
	}
	if j1.Compensation.MaxAmount == nil || *j1.Compensation.MaxAmount != 500000 {
		t.Errorf("job[0].Compensation.MaxAmount = %v, want 500000", j1.Compensation.MaxAmount)
	}
	if j1.DatePosted == nil {
		t.Error("job[0].DatePosted is nil, expected a parsed date")
	}
	if j1.ApplyMethod != "external_url" {
		t.Errorf("job[0].ApplyMethod = %q, want %q", j1.ApplyMethod, "external_url")
	}

	// Check second job
	j2 := jobs[1]
	if j2.ID != "habrcareer-67890" {
		t.Errorf("job[1].ID = %q, want %q", j2.ID, "habrcareer-67890")
	}
	if j2.Title != "Go Backend Engineer" {
		t.Errorf("job[1].Title = %q, want %q", j2.Title, "Go Backend Engineer")
	}
	if j2.JobURL != "https://career.habr.com/vacancy/67890/" {
		t.Errorf("job[1].JobURL = %q", j2.JobURL)
	}
	if j2.IsRemote {
		t.Error("job[1].IsRemote should be false")
	}
	if j2.JobType != "parttime" {
		t.Errorf("job[1].JobType = %q, want %q", j2.JobType, "parttime")
	}
	if j2.Location.City != "Saint Petersburg" {
		t.Errorf("job[1].Location.City = %q, want %q", j2.Location.City, "Saint Petersburg")
	}
	if j2.Compensation == nil {
		t.Fatal("job[1].Compensation should not be nil")
	}
	if j2.Compensation.MinAmount != nil {
		t.Error("job[1].Compensation.MinAmount should be nil (only 'to' provided)")
	}
	if j2.Compensation.MaxAmount == nil || *j2.Compensation.MaxAmount != 200000 {
		t.Errorf("job[1].Compensation.MaxAmount = %v, want 200000", j2.Compensation.MaxAmount)
	}
}

func TestScraper_FailsOnEmptyResponse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"list":[],"meta":{"totalResults":0}}`))
	}))
	defer ts.Close()

	s := NewWithAPIURL(nil, ts.URL)
	_, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 25})
	if err == nil {
		t.Fatal("expected error for empty list, got nil")
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

func TestScraper_Scrape_SearchTerm(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify search term is passed as query param
		if r.URL.Query().Get("q") != "golang" {
			t.Errorf("expected q=golang, got %s", r.URL.Query().Get("q"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(testResponse))
	}))
	defer ts.Close()

	s := NewWithAPIURL(nil, ts.URL)
	jobs, err := s.Scrape(context.Background(), model.ScraperInput{
		SearchTerm:    "golang",
		ResultsWanted: 25,
	})
	if err != nil {
		t.Fatalf("Scrape() returned error: %v", err)
	}

	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(jobs))
	}
}

func TestScraper_ResultsWanted(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(testResponse))
	}))
	defer ts.Close()

	s := NewWithAPIURL(nil, ts.URL)
	jobs, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 1})
	if err != nil {
		t.Fatalf("Scrape() returned error: %v", err)
	}

	if len(jobs) != 1 {
		t.Fatalf("expected 1 job with ResultsWanted=1, got %d", len(jobs))
	}
}
