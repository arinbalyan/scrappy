package careerjet_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	sut "github.com/arinbalyan/scrappy/internal/scraper/careerjet"
)

// sampleCareerJetResponse simulates a CareerJet API response with three job listings.
const sampleCareerJetResponse = `{
  "type": "SUCCESS",
  "hits": 3,
  "pages": 1,
  "response_time": 42,
  "jobs": [
    {
      "title": "Senior Software Engineer",
      "company": "Google",
      "date": "2026-05-20",
      "description": "We are looking for a senior software engineer to join our team.",
      "locations": "San Francisco, CA",
      "url": "https://careerjet.com/job/abc123",
      "site": "careerjet",
      "salary": "$150,000 - $200,000",
      "salary_min": 150000,
      "salary_max": 200000,
      "salary_type": "Y",
      "salary_currency_code": "USD"
    },
    {
      "title": "Machine Learning Engineer",
      "company": "OpenAI",
      "date": "2026-05-19",
      "description": "Join our AI research team building cutting-edge models.",
      "locations": "New York, NY",
      "url": "https://careerjet.com/job/def456",
      "site": "careerjet",
      "salary": "$180,000 - $250,000",
      "salary_min": 180000,
      "salary_max": 250000,
      "salary_type": "Y",
      "salary_currency_code": "USD"
    },
    {
      "title": "Backend Developer",
      "company": "Acme Corp",
      "date": "2026-05-18",
      "description": "Build scalable microservices for our platform.",
      "locations": "Remote, US",
      "url": "https://careerjet.com/job/ghi789",
      "site": "careerjet",
      "salary": 0,
      "salary_min": 0,
      "salary_max": 0,
      "salary_type": "",
      "salary_currency_code": ""
    }
  ]
}`

func TestCareerJetParsesJobs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(sampleCareerJetResponse))
	}))
	defer srv.Close()

	s := sut.NewWithURLs(srv.Client(), srv.URL, "test-affid")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	jobs, err := s.Scrape(ctx, model.ScraperInput{SearchTerm: "engineer", ResultsWanted: 3})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(jobs) != 3 {
		t.Fatalf("expected 3 jobs, got %d", len(jobs))
	}

	// Verify first job fields.
	expected := []struct {
		title   string
		company string
		city    string
		state   string
	}{
		{"Senior Software Engineer", "Google", "San Francisco", "CA"},
		{"Machine Learning Engineer", "OpenAI", "New York", "NY"},
		{"Backend Developer", "Acme Corp", "Remote", "US"},
	}
	for i, exp := range expected {
		if jobs[i].Title != exp.title {
			t.Errorf("job[%d] title = %q, want %q", i, jobs[i].Title, exp.title)
		}
		if jobs[i].CompanyName != exp.company {
			t.Errorf("job[%d] company = %q, want %q", i, jobs[i].CompanyName, exp.company)
		}
		if jobs[i].Location.City != exp.city {
			t.Errorf("job[%d] city = %q, want %q", i, jobs[i].Location.City, exp.city)
		}
		if jobs[i].Location.State != exp.state {
			t.Errorf("job[%d] state = %q, want %q", i, jobs[i].Location.State, exp.state)
		}
	}

	// Verify job URLs are non-empty and IDs are generated.
	for i, j := range jobs {
		if j.JobURL == "" {
			t.Errorf("job[%d] has empty JobURL", i)
		}
		if len(j.ID) < 5 {
			t.Errorf("job[%d] ID too short: %q", i, j.ID)
		}
	}

	// Verify first job has compensation.
	if jobs[0].Compensation == nil {
		t.Fatal("expected job[0] to have compensation")
	}
	if jobs[0].Compensation.MinAmount == nil || *jobs[0].Compensation.MinAmount != 150000 {
		t.Errorf("job[0] min amount = %v, want 150000", jobs[0].Compensation.MinAmount)
	}
	if jobs[0].Compensation.MaxAmount == nil || *jobs[0].Compensation.MaxAmount != 200000 {
		t.Errorf("job[0] max amount = %v, want 200000", jobs[0].Compensation.MaxAmount)
	}
	if jobs[0].Compensation.Interval != model.IntervalYearly {
		t.Errorf("job[0] interval = %q, want yearly", jobs[0].Compensation.Interval)
	}

	// Verify third job has no compensation (salary_min was 0).
	if jobs[2].Compensation != nil {
		t.Error("expected job[2] to have nil compensation (salary_min was 0)")
	}
}

func TestCareerJetFailsOn429(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	s := sut.NewWithURLs(srv.Client(), srv.URL, "test-affid")
	_, err := s.Scrape(context.Background(), model.ScraperInput{SearchTerm: "engineer", ResultsWanted: 1})
	if err == nil {
		t.Fatal("expected error for 429 status, got nil")
	}
}

func TestCareerJetFailsOn503(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	s := sut.NewWithURLs(srv.Client(), srv.URL, "test-affid")
	_, err := s.Scrape(context.Background(), model.ScraperInput{SearchTerm: "engineer", ResultsWanted: 1})
	if err == nil {
		t.Fatal("expected error for 503 status, got nil")
	}
}

func TestCareerJetEmptyResponse(t *testing.T) {
	body := `{"type": "SUCCESS", "hits": 0, "pages": 0, "response_time": 10, "jobs": []}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	s := sut.NewWithURLs(srv.Client(), srv.URL, "test-affid")
	_, err := s.Scrape(context.Background(), model.ScraperInput{SearchTerm: "nonexistent", ResultsWanted: 1})
	if err == nil {
		t.Fatal("expected error for empty response, got nil")
	}
}

func TestCareerJetContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // immediately cancel

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(sampleCareerJetResponse))
	}))
	defer srv.Close()

	s := sut.NewWithURLs(srv.Client(), srv.URL, "test-affid")
	_, err := s.Scrape(ctx, model.ScraperInput{SearchTerm: "engineer", ResultsWanted: 3})
	if err == nil {
		t.Fatal("expected error for cancelled context, got nil")
	}
}
