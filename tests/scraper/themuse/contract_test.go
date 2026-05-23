package themuse_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	sut "github.com/arinbalyan/scrappy/internal/scraper/themuse"
)

func TestTheMuseHappyPath(t *testing.T) {
	body := `{
		"page": 0, "page_count": 1, "total": 2,
		"results": [
			{
				"id": 1,
				"name": "Software Engineer",
				"company": {"id": 10, "short_name": "acme", "name": "Acme Corp"},
				"locations": [{"name": "San Francisco, CA, USA"}],
				"levels": [{"name": "Senior", "short_name": "senior"}],
				"refs": {"landing_page": "https://themuse.com/jobs/1"},
				"publication_date": "2024-01-15T12:00:00Z",
				"contents": "<p>Job description</p>",
				"model_type": "job"
			},
			{
				"id": 2,
				"name": "AI Engineer",
				"company": {"id": 20, "short_name": "aicomp", "name": "AI Company"},
				"locations": [{"name": "Remote"}],
				"levels": [{"name": "Mid", "short_name": "mid"}],
				"refs": {"landing_page": "https://themuse.com/jobs/2"},
				"publication_date": "2024-01-14",
				"contents": "Another description",
				"model_type": "job"
			}
		]
	}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	s := sut.NewWithAPIURL(srv.Client(), srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	jobs, err := s.Scrape(ctx, model.ScraperInput{SearchTerm: "software", ResultsWanted: 5})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job matching 'software', got %d", len(jobs))
	}
	if jobs[0].ID != "themuse-1" {
		t.Fatalf("expected ID 'themuse-1', got %q", jobs[0].ID)
	}
	if jobs[0].CompanyName != "Acme Corp" {
		t.Fatalf("expected 'Acme Corp', got %q", jobs[0].CompanyName)
	}
	if jobs[0].Location.City != "San Francisco" {
		t.Fatalf("expected 'San Francisco', got %q", jobs[0].Location.City)
	}
	if jobs[0].Seniority != "Senior" {
		t.Fatalf("expected 'Senior', got %q", jobs[0].Seniority)
	}
}

func TestTheMuseErrorHandling429(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	s := sut.NewWithAPIURL(srv.Client(), srv.URL)
	_, err := s.Scrape(ctx, model.ScraperInput{SearchTerm: "engineer", ResultsWanted: 5})
	srv.Close()
	if err == nil {
		t.Fatal("expected error for 429 status")
	}
}

func TestTheMuseErrorHandling503(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	s := sut.NewWithAPIURL(srv.Client(), srv.URL)
	_, err := s.Scrape(ctx, model.ScraperInput{SearchTerm: "engineer", ResultsWanted: 5})
	srv.Close()
	if err == nil {
		t.Fatal("expected error for 503 status")
	}
}

func TestTheMuseEmptyResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"page":0,"page_count":0,"total":0,"results":[]}`))
	}))
	defer srv.Close()
	s := sut.NewWithAPIURL(srv.Client(), srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := s.Scrape(ctx, model.ScraperInput{SearchTerm: "engineer", ResultsWanted: 5})
	if err == nil {
		t.Fatal("expected error for empty response")
	}
}

func TestTheMuseContextCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()
	// Give context time to expire before making the request
	time.Sleep(5 * time.Millisecond)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
			// client cancelled — return nothing
		case <-time.After(100 * time.Millisecond):
			_, _ = w.Write([]byte(`{"results":[{"id":1,"name":"test","refs":{"landing_page":"https://themuse.com/jobs/1"}}]}`))
		}
	}))
	defer srv.Close()

	s := sut.NewWithAPIURL(srv.Client(), srv.URL)
	_, err := s.Scrape(ctx, model.ScraperInput{SearchTerm: "test", ResultsWanted: 5})
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}
