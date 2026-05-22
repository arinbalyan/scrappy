package mycareersfuture_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	sut "github.com/arinbalyan/scrappy/internal/scraper/mycareersfuture"
)

func TestMyCareersFutureHappyPath(t *testing.T) {
	apiResp := map[string]any{
		"results": []map[string]any{
			{
				"uuid":        "job-001",
				"title":       "Software Engineer",
				"description": "<p>Great job opportunity</p>",
				"company":     map[string]any{"name": "Acme Corp"},
				"salary":      map[string]any{"minimum": 5000, "maximum": 8000, "currency": "SGD"},
				"location":    map[string]any{"name": "Singapore"},
				"postedDate":  "2025-01-15T00:00:00Z",
			},
			{
				"uuid":     "job-002",
				"title":    "Data Scientist",
				"company":  map[string]any{"name": "Tech Co"},
				"location": map[string]any{"name": "Singapore"},
			},
		},
	}
	body, _ := json.Marshal(apiResp)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	s := sut.NewWithURLs(srv.Client(), srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	jobs, err := s.Scrape(ctx, model.ScraperInput{SearchTerm: "engineer", ResultsWanted: 2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(jobs))
	}
	if jobs[0].Title != "Software Engineer" {
		t.Fatalf("expected 'Software Engineer', got %q", jobs[0].Title)
	}
	if jobs[1].CompanyName != "Tech Co" {
		t.Fatalf("expected 'Tech Co', got %q", jobs[1].CompanyName)
	}
	if jobs[0].ID != "mcf-job-001" {
		t.Fatalf("expected 'mcf-job-001', got %q", jobs[0].ID)
	}
	if jobs[0].Compensation == nil {
		t.Fatal("expected compensation to be set")
	}
}

func TestMyCareersFutureErrorHandling429(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	s := sut.NewWithURLs(srv.Client(), srv.URL)
	_, err := s.Scrape(context.Background(), model.ScraperInput{SearchTerm: "x", ResultsWanted: 1})
	if err == nil {
		t.Fatal("expected error for 429 status")
	}
}

func TestMyCareersFutureErrorHandling503(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	s := sut.NewWithURLs(srv.Client(), srv.URL)
	_, err := s.Scrape(context.Background(), model.ScraperInput{SearchTerm: "x", ResultsWanted: 1})
	if err == nil {
		t.Fatal("expected error for 503 status")
	}
}

func TestMyCareersFutureEmptyResponse(t *testing.T) {
	apiResp := map[string]any{"results": []any{}}
	body, _ := json.Marshal(apiResp)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	s := sut.NewWithURLs(srv.Client(), srv.URL)
	_, err := s.Scrape(context.Background(), model.ScraperInput{SearchTerm: "x", ResultsWanted: 1})
	if err == nil {
		t.Fatal("expected error for empty response (no meaningful jobs)")
	}
}

func TestMyCareersFutureContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"results":[]}`))
	}))
	defer srv.Close()

	s := sut.NewWithURLs(srv.Client(), srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	// Let the context expire before calling scrape
	time.Sleep(50 * time.Millisecond)

	_, err := s.Scrape(ctx, model.ScraperInput{SearchTerm: "x", ResultsWanted: 1})
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}
