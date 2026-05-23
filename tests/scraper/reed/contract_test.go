package reed_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	sut "github.com/arinbalyan/scrappy/internal/scraper/reed"
)

// validJobsHTML is a minimal HTML page with a __NEXT_DATA__ blob containing 3 jobs.
const validJobsHTML = `<!DOCTYPE html><html><head>
<script id="__NEXT_DATA__" type="application/json">{
  "props": {
    "pageProps": {
      "searchResults": {
        "count": 3,
        "jobs": [
          {
            "jobDetail": {
              "jobId": 1001,
              "jobTitle": "Software Engineer",
              "jobDescription": "We are looking for a software engineer to join our team.",
              "displayDate": "2026-05-20T10:00:00",
              "displayLocationName": "London",
              "salaryFrom": 50000,
              "salaryTo": 80000,
              "salaryType": 5,
              "salaryCurrencyId": 1,
              "salaryDescription": 0,
              "ouName": "Acme Corp",
              "jobType": 1,
              "remoteWorkingOption": "NotSpecified",
              "isFullTime": true
            },
            "url": "/jobs/software-engineer/1001",
            "profileName": "Acme Corp"
          },
          {
            "jobDetail": {
              "jobId": 1002,
              "jobTitle": "Senior Developer",
              "jobDescription": "Senior developer with Go experience needed.",
              "displayDate": "2026-05-19T15:30:00",
              "displayLocationName": "Manchester",
              "salaryFrom": 70000,
              "salaryTo": 95000,
              "salaryType": 5,
              "salaryCurrencyId": 1,
              "salaryDescription": 0,
              "ouName": "Beta Ltd",
              "jobType": 1,
              "remoteWorkingOption": "Hybrid",
              "isFullTime": true
            },
            "url": "/jobs/senior-developer/1002",
            "profileName": "Beta Ltd"
          },
          {
            "jobDetail": {
              "jobId": 1003,
              "jobTitle": "DevOps Engineer",
              "jobDescription": "Join our infrastructure team.",
              "displayDate": "2026-05-18T09:00:00",
              "displayLocationName": "Remote",
              "salaryFrom": 0,
              "salaryTo": 0,
              "salaryType": 5,
              "salaryCurrencyId": 1,
              "salaryDescription": 64,
              "ouName": "Gamma Inc",
              "jobType": 2,
              "remoteWorkingOption": "FullyRemote",
              "isFullTime": false
            },
            "url": "/jobs/devops-engineer/1003",
            "profileName": "Gamma Inc"
          }
        ]
      }
    }
  }
}</script></head><body></body></html>`

// emptyJobsHTML has valid structure but no job listings.
const emptyJobsHTML = `<!DOCTYPE html><html><head>
<script id="__NEXT_DATA__" type="application/json">{
  "props": {
    "pageProps": {
      "searchResults": {
        "count": 0,
        "jobs": []
      }
    }
  }
}</script></head><body></body></html>`

func TestReedHappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(validJobsHTML))
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
	if jobs[0].ID != "reed-1001" {
		t.Errorf("expected ID reed-1001, got %s", jobs[0].ID)
	}
	if jobs[0].Title != "Software Engineer" {
		t.Errorf("expected title 'Software Engineer', got %s", jobs[0].Title)
	}
	if jobs[0].CompanyName != "Acme Corp" {
		t.Errorf("expected company 'Acme Corp', got %s", jobs[0].CompanyName)
	}
	if jobs[0].JobURL != "https://www.reed.co.uk/jobs/software-engineer/1001" {
		t.Errorf("unexpected job URL: %s", jobs[0].JobURL)
	}
	if jobs[0].IsRemote {
		t.Error("expected isRemote=false for NotSpecified")
	}
	if jobs[0].Compensation == nil {
		t.Fatal("expected compensation for job with salary")
	}
	if jobs[0].Compensation.Currency != "GBP" {
		t.Errorf("expected GBP currency, got %s", jobs[0].Compensation.Currency)
	}
	if jobs[0].DatePosted == nil {
		t.Fatal("expected date posted")
	}

	// Validate second job (hybrid)
	if !jobs[1].IsRemote {
		t.Error("expected isRemote=true for Hybrid")
	}

	// Validate third job (no salary, fully remote)
	if jobs[2].Compensation != nil {
		t.Error("expected nil compensation for no-salary job")
	}
	if !jobs[2].IsRemote {
		t.Error("expected isRemote=true for FullyRemote")
	}
	if jobs[2].JobURL != "https://www.reed.co.uk/jobs/devops-engineer/1003" {
		t.Errorf("unexpected job URL: %s", jobs[2].JobURL)
	}
}

func TestReedFailsOn429And503(t *testing.T) {
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

func TestReedEmptyResponseReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(emptyJobsHTML))
	}))
	defer srv.Close()

	s := sut.NewWithURLs(srv.Client(), srv.URL)
	_, err := s.Scrape(context.Background(), model.ScraperInput{SearchTerm: "x", ResultsWanted: 1})
	if err == nil {
		t.Fatal("expected error for empty response")
	}
}

func TestReedContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel before any request

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(validJobsHTML))
	}))
	defer srv.Close()

	s := sut.NewWithURLs(srv.Client(), srv.URL)
	_, err := s.Scrape(ctx, model.ScraperInput{SearchTerm: "engineer", ResultsWanted: 5})
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}
