package fwdayweek_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	sut "github.com/arinbalyan/scrappy/internal/scraper/4dayweek"
)

const sampleResponse = `{
  "jobs": [
    {
      "id": "019e1ba3-6a9d-7c28-ad6e-e5d369cd5376",
      "title": "Ai Engineer",
      "slug": "ai-engineer-haired-cv",
      "company_name": "Haired CV",
      "work_arrangement": "remote",
      "posted": 1778580392,
      "schedule_type": "4_day_week",
      "salary": "$120k - $160k",
      "salary_lower": 12000000,
      "salary_upper": 16000000,
      "salary_currency": "USD",
      "salary_period": "year",
      "category": "engineering",
      "level": "senior",
      "company": {
        "name": "Haired CV",
        "slug": "haired-cv",
        "logo_url": "https://example.com/logo.png"
      },
      "work_life_score": 88
    },
    {
      "id": "019e19b6-a42a-7a87-b2c5-a88d8112c6b0",
      "title": "Senior ML Engineer",
      "slug": "senior-ml-engineer-civo",
      "company_name": "Civo",
      "work_arrangement": "remote",
      "posted": 1778547861,
      "schedule_type": "4_day_week",
      "category": "engineering",
      "level": "senior",
      "company": {
        "name": "Civo",
        "slug": "civo"
      },
      "work_life_score": 88
    }
  ],
  "total": 2,
  "page": 1,
  "has_more": false
}`

// ---------- Happy path ----------

func Test4DayWeekParsesJobs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(sampleResponse))
	}))
	defer srv.Close()

	s := sut.NewWithAPIURL(srv.Client(), srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	jobs, err := s.Scrape(ctx, model.ScraperInput{SearchTerm: "engineer", ResultsWanted: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(jobs))
	}
	if jobs[0].Title != "Ai Engineer" {
		t.Fatalf("expected title 'Ai Engineer', got %q", jobs[0].Title)
	}
	if jobs[0].CompanyName != "Haired CV" {
		t.Fatalf("expected company 'Haired CV', got %q", jobs[0].CompanyName)
	}
	if !jobs[0].IsRemote {
		t.Fatal("expected job to be remote")
	}
	if jobs[0].Compensation == nil {
		t.Fatal("expected compensation to be parsed")
	} else {
		if jobs[0].Compensation.MinAmount == nil || *jobs[0].Compensation.MinAmount != 120000.0 {
			t.Fatalf("expected min salary 120000, got %v", jobs[0].Compensation.MinAmount)
		}
	}
}

// ---------- Error handling 429 and 503 ----------

func Test4DayWeekFailsOn429And503(t *testing.T) {
	for _, status := range []int{http.StatusTooManyRequests, http.StatusServiceUnavailable} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(status)
		}))
		s := sut.NewWithAPIURL(srv.Client(), srv.URL)
		_, err := s.Scrape(context.Background(), model.ScraperInput{SearchTerm: "engineer", ResultsWanted: 5})
		srv.Close()
		if err == nil {
			t.Fatalf("expected error for status %d", status)
		}
	}
}

// ---------- Empty response ----------

func Test4DayWeekFailsOnEmptyResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jobs":[],"total":0,"page":1,"has_more":false}`))
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

// ---------- Context cancellation ----------

func Test4DayWeekContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before any request

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(sampleResponse))
	}))
	defer srv.Close()

	s := sut.NewWithAPIURL(srv.Client(), srv.URL)
	_, err := s.Scrape(ctx, model.ScraperInput{SearchTerm: "engineer", ResultsWanted: 5})
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}
