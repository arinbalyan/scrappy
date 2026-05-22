package jobsdb_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	sut "github.com/arinbalyan/scrappy/internal/scraper/jobsdb"
)

// validJobsJSON is a minimal JobsDB Chalice API response with 3 jobs.
const validJobsJSON = `{
  "data": [
    {
      "id": "job-001",
      "title": "Software Engineer",
      "companyName": "Tech Corp",
      "location": "Singapore",
      "salary": "SGD 5000 - 8000",
      "jobType": "full_time",
      "listingDate": "2026-05-20T10:00:00Z",
      "teaser": "Join our engineering team to build great products.",
      "jobUrl": "/job/software-engineer/001",
      "description": "<p>We are looking for a software engineer.</p>",
      "workType": "on_site",
      "isRemote": false
    },
    {
      "id": "job-002",
      "title": "Senior Developer",
      "companyName": "Beta Ltd",
      "location": "Hong Kong",
      "salary": "HKD 40000 - 60000",
      "jobType": "full_time",
      "listingDate": "2026-05-19T15:30:00Z",
      "description": "Senior developer with Go experience needed.",
      "jobUrl": "/job/senior-developer/002",
      "workType": "remote",
      "isRemote": false
    },
    {
      "id": "job-003",
      "title": "DevOps Engineer",
      "companyName": "Gamma Inc",
      "location": "Remote",
      "listingDate": "2026-05-18T09:00:00Z",
      "description": "Join our infrastructure team.",
      "workType": "on_site",
      "isRemote": true
    }
  ]
}`

// emptyJobsJSON has valid structure but no job listings.
const emptyJobsJSON = `{"data": []}`

func TestJobsDBHappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request parameters
		if r.URL.Query().Get("keywords") != "engineer" {
			t.Errorf("expected keywords=engineer, got %s", r.URL.Query().Get("keywords"))
		}
		if r.URL.Query().Get("siteKey") != "SG-Main" {
			t.Errorf("expected siteKey=SG-Main, got %s", r.URL.Query().Get("siteKey"))
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(validJobsJSON))
	}))
	defer srv.Close()

	s := sut.NewWithURLs(srv.Client(), srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	jobs, err := s.Scrape(ctx, model.ScraperInput{SearchTerm: "engineer", ResultsWanted: 3})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(jobs) != 3 {
		t.Fatalf("expected 3 jobs, got %d", len(jobs))
	}

	// Validate first job fields
	if jobs[0].ID != "jobsdb-job-001" {
		t.Errorf("expected ID jobsdb-job-001, got %s", jobs[0].ID)
	}
	if jobs[0].Title != "Software Engineer" {
		t.Errorf("expected title 'Software Engineer', got %s", jobs[0].Title)
	}
	if jobs[0].CompanyName != "Tech Corp" {
		t.Errorf("expected company 'Tech Corp', got %s", jobs[0].CompanyName)
	}
	if jobs[0].JobURL != "https://www.jobsdb.com/job/software-engineer/001" {
		t.Errorf("unexpected job URL: %s", jobs[0].JobURL)
	}
	if jobs[0].Location.City != "Singapore" {
		t.Errorf("expected location 'Singapore', got %s", jobs[0].Location.City)
	}
	if jobs[0].IsRemote {
		t.Error("expected IsRemote=false for on_site workType")
	}
	if jobs[0].DatePosted == nil {
		t.Fatal("expected date posted for job-001")
	}
	if jobs[0].JobType != "full_time" {
		t.Errorf("expected job type 'full_time', got %s", jobs[0].JobType)
	}

	// Validate second job (remote via workType field)
	if !jobs[1].IsRemote {
		t.Error("expected IsRemote=true for workType=remote")
	}
	if jobs[1].JobURL != "https://www.jobsdb.com/job/senior-developer/002" {
		t.Errorf("expected job URL with prefix, got %s", jobs[1].JobURL)
	}

	// Validate third job (isRemote=true directly, missing optional fields)
	if !jobs[2].IsRemote {
		t.Error("expected IsRemote=true for isRemote=true")
	}
	if jobs[2].JobURL != "https://www.jobsdb.com/job/job-003" {
		t.Errorf("expected fallback job URL, got %s", jobs[2].JobURL)
	}
	if jobs[2].CompanyName != "Gamma Inc" {
		t.Errorf("expected company 'Gamma Inc', got %s", jobs[2].CompanyName)
	}
}

func TestJobsDBFailsOn429And503(t *testing.T) {
	for _, status := range []int{http.StatusTooManyRequests, http.StatusServiceUnavailable} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(status)
		}))
		s := sut.NewWithURLs(srv.Client(), srv.URL)
		_, err := s.Scrape(context.Background(), model.ScraperInput{SearchTerm: "x", ResultsWanted: 1})
		srv.Close()
		if err == nil {
			t.Fatalf("expected error for status %d", status)
		}
	}
}

func TestJobsDBEmptyResponseReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(emptyJobsJSON))
	}))
	defer srv.Close()

	s := sut.NewWithURLs(srv.Client(), srv.URL)
	_, err := s.Scrape(context.Background(), model.ScraperInput{SearchTerm: "x", ResultsWanted: 1})
	if err == nil {
		t.Fatal("expected error for empty response")
	}
}

func TestJobsDBContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel before any request

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(validJobsJSON))
	}))
	defer srv.Close()

	s := sut.NewWithURLs(srv.Client(), srv.URL)
	_, err := s.Scrape(ctx, model.ScraperInput{SearchTerm: "engineer", ResultsWanted: 5})
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}
