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

// validJobsJSON is a minimal SEEK v5 API response with 3 jobs.
const validJobsJSON = `{
  "data": [
    {
      "id": "12345",
      "title": "Software Engineer",
      "advertiser": { "description": "Tech Corp" },
      "teaser": "Join our engineering team to build great products.",
      "listingDate": "2026-05-25T05:29:09Z",
      "locations": [{"label": "Singapore", "countryCode": "SG"}],
      "salaryLabel": "SGD 5,000 – SGD 8,000 per month",
      "workTypes": ["Full time"],
      "workArrangements": {"displayText": "On-site"},
      "classifications": [
        {
          "classification": {"id": "6281", "description": "Information & Communication Technology"},
          "subclassification": {"id": "6287", "description": "Developers/Programmers"}
        }
      ],
      "bulletPoints": ["Great team", "Flexible hours"],
      "branding": {"serpLogoUrl": "https://logo.example.com/corp.png"}
    },
    {
      "id": "67890",
      "title": "Senior Developer",
      "companyName": "Beta Ltd",
      "teaser": "Senior Go developer needed for our platform team.",
      "listingDate": "2026-05-24T15:30:00Z",
      "locations": [{"label": "Hong Kong", "countryCode": "HK"}],
      "salaryLabel": "HKD 40,000 – HKD 60,000 per month",
      "workTypes": ["Full time"],
      "workArrangements": {"displayText": "Remote"},
      "classifications": [
        {
          "classification": {"id": "6281", "description": "Information & Communication Technology"},
          "subclassification": {"id": "6287", "description": "Developers/Programmers"}
        }
      ]
    },
    {
      "id": "11111",
      "title": "DevOps Engineer",
      "companyName": "Gamma Inc",
      "listingDate": "2026-05-23T09:00:00Z",
      "locations": [{"label": "Singapore", "countryCode": "SG"}],
      "workTypes": ["Contract"],
      "workArrangements": {"displayText": "Work from home"}
    }
  ],
  "totalCount": 3
}`

// emptyJobsJSON has valid structure but no job listings.
const emptyJobsJSON = `{"data": [], "totalCount": 0}`

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

	jobs, err := s.Scrape(ctx, model.ScraperInput{SearchTerm: "engineer", ResultsWanted: 5})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(jobs) != 3 {
		t.Fatalf("expected 3 jobs, got %d", len(jobs))
	}

	// Validate first job fields
	if jobs[0].ID != "jobsdb-12345" {
		t.Errorf("expected ID jobsdb-12345, got %s", jobs[0].ID)
	}
	if jobs[0].Title != "Software Engineer" {
		t.Errorf("expected title 'Software Engineer', got %s", jobs[0].Title)
	}
	if jobs[0].CompanyName != "Tech Corp" {
		t.Errorf("expected company 'Tech Corp', got %s", jobs[0].CompanyName)
	}
	if jobs[0].JobURL != "https://www.jobsdb.com/job/12345" {
		t.Errorf("unexpected job URL: %s", jobs[0].JobURL)
	}
	if jobs[0].Location.City != "Singapore" {
		t.Errorf("expected location 'Singapore', got %s", jobs[0].Location.City)
	}
	if jobs[0].IsRemote {
		t.Error("expected IsRemote=false for on-site work arrangement")
	}
	if jobs[0].DatePosted == nil {
		t.Fatal("expected date posted for job-12345")
	}
	if jobs[0].JobType != "Full time" {
		t.Errorf("expected job type 'Full time', got %s", jobs[0].JobType)
	}
	if jobs[0].Compensation == nil {
		t.Fatal("expected compensation")
	}
	if jobs[0].Compensation.Currency != "SGD" {
		t.Errorf("expected SGD currency, got %s", jobs[0].Compensation.Currency)
	}
	if jobs[0].Department != "Developers/Programmers" {
		t.Errorf("expected department 'Developers/Programmers', got %s", jobs[0].Department)
	}
	if jobs[0].Industry != "Information & Communication Technology" {
		t.Errorf("expected industry 'Information & Communication Technology', got %s", jobs[0].Industry)
	}
	if jobs[0].CompanyLogoURL != "https://logo.example.com/corp.png" {
		t.Errorf("unexpected logo URL: %s", jobs[0].CompanyLogoURL)
	}
	if !stringsContains(jobs[0].Description, "Great team") {
		t.Errorf("expected description to contain bullet points, got %s", jobs[0].Description)
	}
	if !stringsContains(jobs[0].Description, "Join our engineering team") {
		t.Errorf("expected description to contain teaser, got %s", jobs[0].Description)
	}

	// Validate second job (remote via workArrangements)
	if !jobs[1].IsRemote {
		t.Error("expected IsRemote=true for workArrangements.displayText=Remote")
	}
	if jobs[1].CompanyName != "Beta Ltd" {
		t.Errorf("expected company 'Beta Ltd', got %s", jobs[1].CompanyName)
	}
	if jobs[1].Location.City != "Hong Kong" {
		t.Errorf("expected location 'Hong Kong', got %s", jobs[1].Location.City)
	}
	if jobs[1].JobURL != "https://www.jobsdb.com/job/67890" {
		t.Errorf("expected job URL https://www.jobsdb.com/job/67890, got %s", jobs[1].JobURL)
	}

	// Validate third job (no salary, no advertiser, contract)
	if jobs[2].IsRemote {
		t.Error("expected IsRemote=false for workArrangements.displayText=Work from home (not strictly Remote)")
	}
	if jobs[2].CompanyName != "Gamma Inc" {
		t.Errorf("expected company 'Gamma Inc', got %s", jobs[2].CompanyName)
	}
	if jobs[2].JobType != "Contract" {
		t.Errorf("expected job type 'Contract', got %s", jobs[2].JobType)
	}
}

func stringsContains(s, substr string) bool {
	return len(s) >= len(substr) && containsStr(s, substr)
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
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
