package headhunter_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	sut "github.com/arinbalyan/scrappy/internal/scraper/headhunter"
)

func TestHeadhunterHappyPath(t *testing.T) {
	srv := newTestServer(http.StatusOK, `{
		"items": [
			{
				"id": "123",
				"name": "Senior Go Developer",
				"area": {"name": "Moscow"},
				"salary": {"from": 300000, "to": 500000, "currency": "RUR"},
				"employer": {"name": "TechCorp"},
				"snippet": {"requirement": "5 years Go", "responsibility": "Build backend services"},
				"alternate_url": "https://hh.ru/vacancy/123",
				"published_at": "2026-05-20T10:00:00+03:00",
				"schedule": {"id": "remote"}
			},
			{
				"id": "456",
				"name": "ML Engineer",
				"area": {"name": "Saint Petersburg"},
				"salary": {"from": 250000, "currency": "RUR"},
				"employer": {"name": "DataFlow"},
				"snippet": {"requirement": "3 years ML"},
				"alternate_url": "https://hh.ru/vacancy/456",
				"published_at": "2026-05-19T14:00:00+03:00"
			}
		],
		"found": 2,
		"pages": 1,
		"per_page": 25
	}`)
	defer srv.Close()

	s := sut.NewWithURLs(srv.Client(), srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	jobs, err := s.Scrape(ctx, model.ScraperInput{SearchTerm: "go developer", ResultsWanted: 5})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(jobs))
	}

	// Verify first job
	j0 := jobs[0]
	if j0.ID != "headhunter-123" {
		t.Errorf("expected id headhunter-123, got %s", j0.ID)
	}
	if j0.Title != "Senior Go Developer" {
		t.Errorf("expected title 'Senior Go Developer', got %s", j0.Title)
	}
	if j0.CompanyName != "TechCorp" {
		t.Errorf("expected company TechCorp, got %s", j0.CompanyName)
	}
	if j0.Location.City != "Moscow" {
		t.Errorf("expected city Moscow, got %s", j0.Location.City)
	}
	if j0.Location.Country != "Russia" {
		t.Errorf("expected country Russia, got %s", j0.Location.Country)
	}
	if !j0.IsRemote {
		t.Error("expected is_remote true")
	}
	if j0.JobURL != "https://hh.ru/vacancy/123" {
		t.Errorf("expected job URL, got %s", j0.JobURL)
	}
	if j0.Compensation == nil {
		t.Fatal("expected compensation")
	}
	if j0.Compensation.Interval != model.IntervalMonthly {
		t.Errorf("expected monthly interval, got %s", j0.Compensation.Interval)
	}
	if j0.Compensation.Currency != "RUR" {
		t.Errorf("expected RUR currency, got %s", j0.Compensation.Currency)
	}
	if j0.Compensation.MinAmount == nil || *j0.Compensation.MinAmount != 300000 {
		t.Errorf("expected min 300000, got %v", j0.Compensation.MinAmount)
	}
	if j0.DatePosted == nil {
		t.Fatal("expected date_posted")
	}

	// Verify second job
	j1 := jobs[1]
	if j1.ID != "headhunter-456" {
		t.Errorf("expected id headhunter-456, got %s", j1.ID)
	}
	if j1.Title != "ML Engineer" {
		t.Errorf("expected title 'ML Engineer', got %s", j1.Title)
	}
	if j1.CompanyName != "DataFlow" {
		t.Errorf("expected company DataFlow, got %s", j1.CompanyName)
	}
	if j1.Location.City != "Saint Petersburg" {
		t.Errorf("expected city Saint Petersburg, got %s", j1.Location.City)
	}
	if j1.IsRemote {
		t.Error("expected is_remote false")
	}
}

func TestHeadhunterErrorHandling429(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	s := sut.NewWithURLs(srv.Client(), srv.URL)
	_, err := s.Scrape(context.Background(), model.ScraperInput{SearchTerm: "go", ResultsWanted: 5})
	if err == nil {
		t.Fatal("expected error for 429 status")
	}
}

func TestHeadhunterErrorHandling503(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	s := sut.NewWithURLs(srv.Client(), srv.URL)
	_, err := s.Scrape(context.Background(), model.ScraperInput{SearchTerm: "go", ResultsWanted: 5})
	if err == nil {
		t.Fatal("expected error for 503 status")
	}
}

func TestHeadhunterEmptyResponse(t *testing.T) {
	srv := newTestServer(http.StatusOK, `{"items":[],"found":0,"pages":0,"per_page":25}`)
	defer srv.Close()

	s := sut.NewWithURLs(srv.Client(), srv.URL)
	_, err := s.Scrape(context.Background(), model.ScraperInput{SearchTerm: "go", ResultsWanted: 5})
	if err == nil {
		t.Fatal("expected error for empty response")
	}
}

func TestHeadhunterContextCancellation(t *testing.T) {
	srv := newTestServer(http.StatusOK, `{
		"items": [
			{
				"id": "123",
				"name": "Senior Go Developer",
				"area": {"name": "Moscow"},
				"salary": {"from": 300000},
				"employer": {"name": "TechCorp"},
				"alternate_url": "https://hh.ru/vacancy/123",
				"published_at": "2026-05-20T10:00:00+03:00"
			}
		],
		"found": 1,
		"pages": 1,
		"per_page": 25
	}`)
	defer srv.Close()

	s := sut.NewWithURLs(srv.Client(), srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	// Give the context time to expire
	time.Sleep(10 * time.Millisecond)

	_, err := s.Scrape(ctx, model.ScraperInput{SearchTerm: "go", ResultsWanted: 5})
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

// newTestServer creates an httptest server that returns fixed status + body.
func newTestServer(status int, body string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		if body != "" {
			// Verify it's valid JSON if it should be
			if json.Valid([]byte(body)) {
				w.Header().Set("Content-Type", "application/json")
			}
			_, _ = w.Write([]byte(body))
		}
	}))
}
