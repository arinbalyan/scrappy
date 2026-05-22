package adzuna_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	sut "github.com/arinbalyan/scrappy/internal/scraper/adzuna"
)

func TestAdzunaParsesJobs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"count": 1,
			"results": [
				{
					"id": "12345",
					"title": "AI Engineer",
					"company": {"display_name": "OpenAI"},
					"redirect_url": "https://openai.com/careers/ai-engineer",
					"location": {"display_name": "San Francisco, CA"},
					"salary_min": 200000,
					"salary_max": 350000,
					"salary_is_predicted": "0",
					"created": "2025-01-15T10:00:00Z",
					"contract_time": "full_time",
					"description": "<p>We are hiring an AI Engineer to build the future.</p>",
					"adref": "ref123"
				}
			]
		}`))
	}))
	defer srv.Close()

	s := sut.NewWithURLs(srv.Client(), srv.URL, "test-id", "test-key")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	jobs, err := s.Scrape(ctx, model.ScraperInput{
		SearchTerm:    "AI Engineer",
		ResultsWanted: 10,
		Country:       model.CountryUSA,
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}

	job := jobs[0]
	if job.ID != "adzuna-12345" {
		t.Errorf("expected ID adzuna-12345, got %s", job.ID)
	}
	if job.Title != "AI Engineer" {
		t.Errorf("expected Title AI Engineer, got %s", job.Title)
	}
	if job.CompanyName != "OpenAI" {
		t.Errorf("expected CompanyName OpenAI, got %s", job.CompanyName)
	}
	if job.JobURL != "https://openai.com/careers/ai-engineer" {
		t.Errorf("unexpected JobURL: %s", job.JobURL)
	}
	if job.Location.City != "San Francisco, CA" {
		t.Errorf("unexpected Location: %s", job.Location.City)
	}
	if job.IsRemote {
		t.Error("expected IsRemote false")
	}
	if job.JobType != "fulltime" {
		t.Errorf("expected JobType fulltime, got %s", job.JobType)
	}
	if job.Compensation == nil {
		t.Fatal("expected Compensation to be set")
	}
	if job.Compensation.Currency != "USD" {
		t.Errorf("expected currency USD, got %s", job.Compensation.Currency)
	}
	if job.Compensation.MinAmount == nil || *job.Compensation.MinAmount != 200000 {
		t.Errorf("expected MinAmount 200000, got %v", job.Compensation.MinAmount)
	}
	if job.Compensation.MaxAmount == nil || *job.Compensation.MaxAmount != 350000 {
		t.Errorf("expected MaxAmount 350000, got %v", job.Compensation.MaxAmount)
	}
	if job.Compensation.Interval != model.IntervalYearly {
		t.Errorf("expected Interval yearly, got %s", job.Compensation.Interval)
	}
	if job.DatePosted == nil {
		t.Fatal("expected DatePosted to be set")
	}
	if !job.DatePosted.Equal(time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)) {
		t.Errorf("unexpected DatePosted: %v", job.DatePosted)
	}
}

func TestAdzunaParsesMultipleJobs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"count": 3,
			"results": [
				{"id":"1","title":"Job One","company":{"display_name":"Co A"},"redirect_url":"https://a.com/1","location":{"display_name":"NYC"},"salary_min":100,"salary_max":200,"salary_is_predicted":"0","created":"2025-01-01T00:00:00Z","contract_time":"full_time","description":"","adref":"r1"},
				{"id":"2","title":"Job Two","company":{"display_name":"Co B"},"redirect_url":"https://b.com/2","location":{"display_name":"SF"},"salary_min":null,"salary_max":null,"salary_is_predicted":"1","created":"2025-01-02T00:00:00Z","contract_time":"part_time","description":"","adref":"r2"},
				{"id":"3","title":"Job Three","company":{"display_name":"Co C"},"redirect_url":"https://c.com/3","location":{"display_name":"Remote"},"salary_min":null,"salary_max":null,"salary_is_predicted":"1","created":"2025-01-03T00:00:00Z","contract_time":"contract","description":"","adref":"r3"}
			]
		}`))
	}))
	defer srv.Close()

	s := sut.NewWithURLs(srv.Client(), srv.URL, "test-id", "test-key")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	jobs, err := s.Scrape(ctx, model.ScraperInput{SearchTerm: "job", ResultsWanted: 10})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(jobs) != 3 {
		t.Fatalf("expected 3 jobs, got %d", len(jobs))
	}

	// Job 1: full_time, has salary
	if jobs[0].JobType != "fulltime" {
		t.Errorf("job 0 expected fulltime, got %s", jobs[0].JobType)
	}
	if jobs[0].Compensation == nil {
		t.Error("job 0 expected compensation")
	}

	// Job 2: part_time, salary predicted → no compensation
	if jobs[1].JobType != "parttime" {
		t.Errorf("job 1 expected parttime, got %s", jobs[1].JobType)
	}
	if jobs[1].Compensation != nil {
		t.Error("job 1 expected nil compensation (salary_is_predicted=1)")
	}

	// Job 3: contract
	if jobs[2].JobType != "contract" {
		t.Errorf("job 2 expected contract, got %s", jobs[2].JobType)
	}
}

func TestAdzunaFailsOn429And503(t *testing.T) {
	for _, status := range []int{http.StatusTooManyRequests, http.StatusServiceUnavailable} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(status)
		}))
		s := sut.NewWithURLs(srv.Client(), srv.URL, "test-id", "test-key")
		_, err := s.Scrape(context.Background(), model.ScraperInput{SearchTerm: "x", ResultsWanted: 1})
		srv.Close()
		if err == nil {
			t.Fatalf("expected error for status %d", status)
		}
	}
}

func TestAdzunaEmptyResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"count":0,"results":[]}`))
	}))
	defer srv.Close()

	s := sut.NewWithURLs(srv.Client(), srv.URL, "test-id", "test-key")
	_, err := s.Scrape(context.Background(), model.ScraperInput{SearchTerm: "x", ResultsWanted: 1})
	if err == nil {
		t.Fatal("expected error for empty response")
	}
}

func TestAdzunaMissingCredentials(t *testing.T) {
	s := sut.NewWithURLs(nil, "http://example.com", "", "")
	_, err := s.Scrape(context.Background(), model.ScraperInput{SearchTerm: "x"})
	if err == nil {
		t.Fatal("expected error for missing credentials")
	}
}

func TestAdzunaDeduplicatesByID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"count": 2,
			"results": [
				{"id":"same-id","title":"First","company":{"display_name":"Co"},"redirect_url":"https://a.com/1","location":{"display_name":"NYC"},"created":"2025-01-01T00:00:00Z","description":"","adref":"r1"},
				{"id":"same-id","title":"Second","company":{"display_name":"Co"},"redirect_url":"https://a.com/2","location":{"display_name":"NYC"},"created":"2025-01-01T00:00:00Z","description":"","adref":"r2"}
			]
		}`))
	}))
	defer srv.Close()

	s := sut.NewWithURLs(srv.Client(), srv.URL, "test-id", "test-key")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	jobs, err := s.Scrape(ctx, model.ScraperInput{SearchTerm: "test", ResultsWanted: 5})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job after dedup, got %d", len(jobs))
	}
	if jobs[0].ID != "adzuna-same-id" {
		t.Errorf("expected adzuna-same-id, got %s", jobs[0].ID)
	}
	// The first one with title "First" should have been kept.
	if jobs[0].Title != "First" {
		t.Errorf("expected title First, got %s", jobs[0].Title)
	}
}

func TestAdzunaSkipsEmptyTitleOrURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"count": 3,
			"results": [
				{"id":"1","title":"Valid","company":{"display_name":"Co"},"redirect_url":"https://a.com/1","location":{"display_name":"NYC"},"created":"2025-01-01T00:00:00Z","description":"","adref":"r1"},
				{"id":"2","title":"","company":{"display_name":"Co"},"redirect_url":"https://a.com/2","location":{"display_name":"NYC"},"created":"2025-01-01T00:00:00Z","description":"","adref":"r2"},
				{"id":"3","title":"No URL","company":{"display_name":"Co"},"redirect_url":"","location":{"display_name":"NYC"},"created":"2025-01-01T00:00:00Z","description":"","adref":"r3"}
			]
		}`))
	}))
	defer srv.Close()

	s := sut.NewWithURLs(srv.Client(), srv.URL, "test-id", "test-key")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	jobs, err := s.Scrape(ctx, model.ScraperInput{SearchTerm: "test", ResultsWanted: 5})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 valid job, got %d", len(jobs))
	}
}

func TestAdzunaHTMLStripping(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"count": 1,
			"results": [
				{
					"id":"1",
					"title":"Test",
					"company":{"display_name":"Co"},
					"redirect_url":"https://a.com/1",
					"location":{"display_name":"NYC"},
					"created":"2025-01-01T00:00:00Z",
					"description":"<p>Hello <b>world</b></p><ul><li>item1</li><li>item2</li></ul>",
					"adref":"r1"
				}
			]
		}`))
	}))
	defer srv.Close()

	s := sut.NewWithURLs(srv.Client(), srv.URL, "test-id", "test-key")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	jobs, err := s.Scrape(ctx, model.ScraperInput{SearchTerm: "test", ResultsWanted: 5})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
	expected := "Hello world item1 item2"
	if jobs[0].Description != expected {
		t.Errorf("expected description %q, got %q", expected, jobs[0].Description)
	}
}

func TestAdzunaContextCancelation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate slow response
		time.Sleep(500 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"count":1,"results":[{"id":"1","title":"Late","company":{"display_name":"Co"},"redirect_url":"https://a.com/1","location":{"display_name":"NYC"},"created":"2025-01-01T00:00:00Z","description":"","adref":"r1"}]}`))
	}))
	defer srv.Close()

	s := sut.NewWithURLs(srv.Client(), srv.URL, "test-id", "test-key")
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	// Give the context time to expire before calling.
	time.Sleep(10 * time.Millisecond)

	_, err := s.Scrape(ctx, model.ScraperInput{SearchTerm: "test", ResultsWanted: 5})
	if err == nil {
		t.Fatal("expected context deadline exceeded error")
	}
}
