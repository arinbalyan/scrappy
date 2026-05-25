package jobstreet_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	sut "github.com/arinbalyan/scrappy/internal/scraper/jobstreet"
)

// validResponse is a SEEK v5 API response with 3 jobs.
const validResponse = `{
  "data": [
    {
      "id": "12345",
      "title": "Software Engineer",
      "advertiser": { "description": "Acme Corp" },
      "teaser": "We are hiring a software engineer for our Kuala Lumpur office.",
      "listingDate": "2026-05-25T10:00:00Z",
      "locations": [{"label": "Kuala Lumpur", "countryCode": "MY"}],
      "salaryLabel": "RM 5,000 – RM 8,000 per month",
      "workTypes": ["Full time"],
      "workArrangements": {"displayText": "On-site"},
      "classifications": [
        {
          "classification": {"id": "6281", "description": "Information & Communication Technology"},
          "subclassification": {"id": "6287", "description": "Developers/Programmers"}
        }
      ],
      "bulletPoints": ["Great culture", "Stock options"]
    },
    {
      "id": "67890",
      "title": "Senior Developer",
      "companyName": "Beta Sdn Bhd",
      "teaser": "Senior Go developer needed for our platform team.",
      "listingDate": "2026-05-24T10:00:00Z",
      "locations": [{"label": "Penang", "countryCode": "MY"}],
      "salaryLabel": "RM 10,000 – RM 15,000 per month",
      "workTypes": ["Full time"],
      "workArrangements": {"displayText": "Remote"},
      "classifications": [
        {
          "classification": {"id": "6281", "description": "Information & Communication Technology"}
        }
      ]
    },
    {
      "id": "11111",
      "title": "DevOps Engineer",
      "companyName": "Gamma Tech",
      "listingDate": "2026-05-23T08:00:00Z",
      "locations": [{"label": "Singapore", "countryCode": "SG"}],
      "workTypes": ["Contract"],
      "workArrangements": {"displayText": "On-site"}
    }
  ],
  "totalCount": 3
}`

// emptyResponse is a valid object with an empty data array.
const emptyResponse = `{"data": [], "totalCount": 0}`

func TestJobStreetHappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request parameters
		if r.URL.Query().Get("siteKey") != "MY-Main" {
			t.Errorf("expected siteKey=MY-Main, got %s", r.URL.Query().Get("siteKey"))
		}
		if r.URL.Query().Get("keywords") != "engineer" {
			t.Errorf("expected keywords=engineer, got %s", r.URL.Query().Get("keywords"))
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(validResponse))
	}))
	defer srv.Close()

	s := sut.NewWithURLs(srv.Client(), srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	jobs, err := s.Scrape(ctx, model.ScraperInput{SearchTerm: "engineer", ResultsWanted: 5})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(jobs) != 3 {
		t.Fatalf("expected 3 jobs, got %d", len(jobs))
	}

	// Validate first job
	if jobs[0].ID != "jobstreet-12345" {
		t.Errorf("expected ID jobstreet-12345, got %s", jobs[0].ID)
	}
	if jobs[0].Title != "Software Engineer" {
		t.Errorf("expected title 'Software Engineer', got %s", jobs[0].Title)
	}
	if jobs[0].CompanyName != "Acme Corp" {
		t.Errorf("expected company 'Acme Corp', got %s", jobs[0].CompanyName)
	}
	if jobs[0].JobURL != "https://www.jobstreet.com/job/12345" {
		t.Errorf("expected job URL https://www.jobstreet.com/job/12345, got %s", jobs[0].JobURL)
	}
	if !stringsContains(jobs[0].Description, "We are hiring a software engineer") {
		t.Errorf("unexpected description: %s", jobs[0].Description)
	}
	if !stringsContains(jobs[0].Description, "Great culture") {
		t.Errorf("expected description to contain bullet points, got %s", jobs[0].Description)
	}
	if jobs[0].Location.City != "Kuala Lumpur" {
		t.Errorf("expected city 'Kuala Lumpur', got %s", jobs[0].Location.City)
	}
	if jobs[0].IsRemote {
		t.Error("expected isRemote=false")
	}
	if jobs[0].Compensation == nil {
		t.Fatal("expected compensation")
	}
	if jobs[0].Compensation.Currency != "RM" {
			t.Errorf("expected RM currency, got %s", jobs[0].Compensation.Currency)
	}
	if jobs[0].DatePosted == nil {
		t.Fatal("expected date posted")
	}
	if jobs[0].JobType != "Full time" {
		t.Errorf("expected job type 'Full time', got %s", jobs[0].JobType)
	}
	if jobs[0].Department != "Developers/Programmers" {
		t.Errorf("expected department 'Developers/Programmers', got %s", jobs[0].Department)
	}
	if jobs[0].Industry != "Information & Communication Technology" {
		t.Errorf("expected industry 'Information & Communication Technology', got %s", jobs[0].Industry)
	}

	// Validate second job (remote)
	if !jobs[1].IsRemote {
		t.Error("expected isRemote=true for workArrangements.displayText=Remote")
	}
	if jobs[1].CompanyName != "Beta Sdn Bhd" {
		t.Errorf("expected company 'Beta Sdn Bhd', got %s", jobs[1].CompanyName)
	}
	if jobs[1].Location.City != "Penang" {
		t.Errorf("expected city 'Penang', got %s", jobs[1].Location.City)
	}
	if jobs[1].JobURL != "https://www.jobstreet.com/job/67890" {
		t.Errorf("expected job URL https://www.jobstreet.com/job/67890, got %s", jobs[1].JobURL)
	}

	// Validate third job
	if jobs[2].CompanyName != "Gamma Tech" {
		t.Errorf("expected company 'Gamma Tech', got %s", jobs[2].CompanyName)
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

func TestJobStreetFailsOn429(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	s := sut.NewWithURLs(srv.Client(), srv.URL)
	_, err := s.Scrape(context.Background(), model.ScraperInput{SearchTerm: "x", ResultsWanted: 1})
	if err == nil {
		t.Fatal("expected error for status 429")
	}
}

func TestJobStreetFailsOn503(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	s := sut.NewWithURLs(srv.Client(), srv.URL)
	_, err := s.Scrape(context.Background(), model.ScraperInput{SearchTerm: "x", ResultsWanted: 1})
	if err == nil {
		t.Fatal("expected error for status 503")
	}
}

func TestJobStreetEmptyResponseReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(emptyResponse))
	}))
	defer srv.Close()

	s := sut.NewWithURLs(srv.Client(), srv.URL)
	_, err := s.Scrape(context.Background(), model.ScraperInput{SearchTerm: "x", ResultsWanted: 1})
	if err == nil {
		t.Fatal("expected error for empty response")
	}
}

func TestJobStreetContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel before any request.

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(validResponse))
	}))
	defer srv.Close()

	s := sut.NewWithURLs(srv.Client(), srv.URL)
	_, err := s.Scrape(ctx, model.ScraperInput{SearchTerm: "engineer", ResultsWanted: 5})
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}
