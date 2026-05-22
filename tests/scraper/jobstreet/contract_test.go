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

// validResponse is a realistic JobStreet API response with 3 jobs.
const validResponse = `[
  {
    "id": "12345",
    "title": "Software Engineer",
    "advertiser": { "description": "Acme Corp" },
    "listingUrl": "https://www.jobstreet.com/job/12345",
    "teaser": "We are hiring a software engineer for our Kuala Lumpur office.",
    "locationWhereValue": "Kuala Lumpur",
    "salary": "MYR 5,000 - MYR 8,000",
    "listingDate": "2026-05-20",
    "isRemote": false,
    "workType": "full_time",
    "classification": { "description": "Engineering" }
  },
  {
    "id": "67890",
    "title": "Senior Developer",
    "companyName": "Beta Sdn Bhd",
    "listingUrl": "https://www.jobstreet.com/job/67890",
    "teaser": "Senior Go developer needed for our platform team.",
    "location": "Penang",
    "salary": "MYR 10,000 - MYR 15,000",
    "listingDate": "2026-05-19T10:00:00Z",
    "isRemote": true
  },
  {
    "id": "abc123",
    "title": "DevOps Engineer",
    "company": "Gamma Tech",
    "jobUrl": "https://www.jobstreet.com/job/abc123",
    "description": "Join our infrastructure team in Singapore.",
    "locationWhereValue": "Singapore",
    "isRemote": false
  }
]`

// validResponseAsObject wraps the same 3 jobs in an object with a "data" field.
const validResponseAsObject = `{
  "data": [
    {
      "id": "111",
      "title": "Frontend Developer",
      "advertiser": { "description": "Delta Inc" },
      "listingUrl": "https://www.jobstreet.com/job/111",
      "salary": "SGD 6,000 - SGD 9,000",
      "listingDate": "2026-05-18",
      "isRemote": false
    },
    {
      "id": "222",
      "title": "Backend Engineer",
      "companyName": "Epsilon Pte Ltd",
      "listingUrl": "https://www.jobstreet.com/job/222",
      "teaser": "Backend engineer with Go experience.",
      "locationWhereValue": "Johor Bahru",
      "isRemote": true
    }
  ]
}`

// emptyResponseArray is a valid JSON array with zero jobs.
const emptyResponseArray = `[]`

// emptyResponseObject is a valid JSON object with an empty data array.
const emptyResponseObject = `{"data": [], "total": 0}`

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
	if jobs[0].Description != "We are hiring a software engineer for our Kuala Lumpur office." {
		t.Errorf("unexpected description")
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
	if jobs[0].Compensation.Currency != "MYR" {
		t.Errorf("expected MYR currency, got %s", jobs[0].Compensation.Currency)
	}
	if jobs[0].DatePosted == nil {
		t.Fatal("expected date posted")
	}
	if jobs[0].JobType != "full_time" {
		t.Errorf("expected job_type full_time, got %s", jobs[0].JobType)
	}
	if jobs[0].Department != "Engineering" {
		t.Errorf("expected department Engineering, got %s", jobs[0].Department)
	}

	// Validate second job (remote)
	if !jobs[1].IsRemote {
		t.Error("expected isRemote=true")
	}
	if jobs[1].CompanyName != "Beta Sdn Bhd" {
		t.Errorf("expected company 'Beta Sdn Bhd', got %s", jobs[1].CompanyName)
	}

	// Validate third job (uses "company" field, "jobUrl" not "listingUrl", "description" not "teaser")
	if jobs[2].CompanyName != "Gamma Tech" {
		t.Errorf("expected company 'Gamma Tech', got %s", jobs[2].CompanyName)
	}
	if jobs[2].Description != "Join our infrastructure team in Singapore." {
		t.Errorf("unexpected description for third job")
	}
	if jobs[2].Location.City != "Singapore" {
		t.Errorf("expected city 'Singapore', got %s", jobs[2].Location.City)
	}
}

func TestJobStreetAcceptsObjectResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(validResponseAsObject))
	}))
	defer srv.Close()

	s := sut.NewWithURLs(srv.Client(), srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	jobs, err := s.Scrape(ctx, model.ScraperInput{SearchTerm: "developer", ResultsWanted: 5})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs from object response, got %d", len(jobs))
	}
	if jobs[0].Title != "Frontend Developer" {
		t.Errorf("expected title 'Frontend Developer', got %s", jobs[0].Title)
	}
	if jobs[0].CompanyName != "Delta Inc" {
		t.Errorf("expected company 'Delta Inc', got %s", jobs[0].CompanyName)
	}
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

func TestJobStreetEmptyArrayReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(emptyResponseArray))
	}))
	defer srv.Close()

	s := sut.NewWithURLs(srv.Client(), srv.URL)
	_, err := s.Scrape(context.Background(), model.ScraperInput{SearchTerm: "x", ResultsWanted: 1})
	if err == nil {
		t.Fatal("expected error for empty array response")
	}
}

func TestJobStreetEmptyObjectReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(emptyResponseObject))
	}))
	defer srv.Close()

	s := sut.NewWithURLs(srv.Client(), srv.URL)
	_, err := s.Scrape(context.Background(), model.ScraperInput{SearchTerm: "x", ResultsWanted: 1})
	if err == nil {
		t.Fatal("expected error for empty object response")
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
