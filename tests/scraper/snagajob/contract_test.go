package snagajob_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	sut "github.com/arinbalyan/scrappy/internal/scraper/snagajob"
)

func TestSnagajobHappyPath(t *testing.T) {
	apiResp := map[string]any{
		"jobs": []map[string]any{
			{
				"id":          101,
				"title":       "Cashier",
				"company":     "Acme Store",
				"url":         "/jobs/101",
				"location":    "Austin",
				"state":       "TX",
				"payMin":      15.0,
				"payMax":      18.0,
				"description": "Ring up customers and stock shelves.",
				"postedDate":  "2026-05-20",
				"jobType":     "Part-Time",
			},
			{
				"id":          102,
				"title":       "Warehouse Associate",
				"companyName": "Big Warehouse Co",
				"detailUrl":   "/jobs/102",
				"city":        "Dallas",
				"state":       "TX",
				"payMin":      20.0,
				"payMax":      25.0,
				"snippet":     "Move boxes and operate forklifts.",
				"datePosted":  "2026-05-19",
				"jobType":     "Full-Time",
			},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(apiResp)
	}))
	defer srv.Close()

	s := sut.NewWithURLs(srv.Client(), srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	jobs, err := s.Scrape(ctx, model.ScraperInput{SearchTerm: "cashier", ResultsWanted: 2})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(jobs))
	}

	// Verify first job fields
	if jobs[0].Title != "Cashier" {
		t.Errorf("expected title 'Cashier', got %q", jobs[0].Title)
	}
	if jobs[0].CompanyName != "Acme Store" {
		t.Errorf("expected company 'Acme Store', got %q", jobs[0].CompanyName)
	}
	if !strings.HasPrefix(jobs[0].ID, "snagajob-") {
		t.Errorf("expected ID starting with 'snagajob-', got %q", jobs[0].ID)
	}
	if jobs[0].JobType != "part-time" {
		t.Errorf("expected job_type 'part-time', got %q", jobs[0].JobType)
	}
	if jobs[0].Compensation == nil {
		t.Fatal("expected non-nil compensation")
	}
	if jobs[0].Compensation.Interval != model.IntervalHourly {
		t.Errorf("expected hourly interval, got %s", jobs[0].Compensation.Interval)
	}
	if jobs[0].Location.City != "Austin" {
		t.Errorf("expected city 'Austin', got %q", jobs[0].Location.City)
	}

	// Verify second job (uses alternate field names)
	if jobs[1].Title != "Warehouse Associate" {
		t.Errorf("expected title 'Warehouse Associate', got %q", jobs[1].Title)
	}
	if jobs[1].CompanyName != "Big Warehouse Co" {
		t.Errorf("expected company 'Big Warehouse Co', got %q", jobs[1].CompanyName)
	}
}

func TestSnagajobFailsOn429And503(t *testing.T) {
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

func TestSnagajobEmptyResponseReturnsError(t *testing.T) {
	apiResp := map[string]any{
		"jobs": []any{},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(apiResp)
	}))
	defer srv.Close()

	s := sut.NewWithURLs(srv.Client(), srv.URL)
	_, err := s.Scrape(context.Background(), model.ScraperInput{SearchTerm: "x", ResultsWanted: 1})
	if err == nil {
		t.Fatal("expected error for empty response")
	}
}

func TestSnagajobContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Sleep longer than the context timeout
		time.Sleep(500 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"jobs":[]}`))
	}))
	defer srv.Close()

	s := sut.NewWithURLs(srv.Client(), srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := s.Scrape(ctx, model.ScraperInput{SearchTerm: "x", ResultsWanted: 1})
	if err == nil {
		t.Fatal("expected error for context cancellation")
	}
}
