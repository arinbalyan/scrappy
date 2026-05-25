package echojobs_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	sut "github.com/arinbalyan/scrappy/internal/scraper/echojobs"
)

func TestEchoJobsFailsOnEmptyResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	s := sut.NewWithAPIURL(srv.Client(), srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	jobs, err := s.Scrape(ctx, model.ScraperInput{SearchTerm: "software engineer", ResultsWanted: 5})
	if err == nil && len(jobs) == 0 {
		t.Fatal("expected non-empty jobs or explicit error for empty upstream response")
	}
}

func TestEchoJobsFailsOn429And503(t *testing.T) {
	for _, status := range []int{http.StatusTooManyRequests, http.StatusServiceUnavailable} {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(status)
		}))
		s := sut.NewWithAPIURL(srv.Client(), srv.URL)
		_, err := s.Scrape(ctx, model.ScraperInput{SearchTerm: "software engineer", ResultsWanted: 5})
		srv.Close()
		cancel()
		if err == nil {
			t.Fatalf("expected error for status %d", status)
		}
	}
}

func TestEchoJobsScraper_ScrapeBasic(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		body := `[
			{
				"id": 1001,
				"title": "AI Engineer",
				"company": "Acme AI",
				"company_logo": "https://example.com/logo.png",
				"url": "https://example.com/jobs/1001",
				"description": "Build machine learning models",
				"location": "San Francisco, CA",
				"salary_min": 120000,
				"salary_max": 180000,
				"salary_currency": "USD",
				"tags": ["python", "tensorflow", "ai"],
				"date_posted": "2026-05-20T10:00:00Z",
				"is_remote": true,
				"job_type": "fulltime"
			},
			{
				"id": "2002",
				"title": "ML Engineer",
				"company": "Brain Corp",
				"url": "https://braincorp.com/jobs/2002",
				"description": "Deep learning research",
				"location": "New York, NY",
				"salary_min": 150000,
				"salary_max": 220000,
				"tags": ["python", "pytorch"],
				"published_at": "2026-05-19",
				"remote": true,
				"job_type": "fulltime"
			}
		]`
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	s := sut.NewWithAPIURL(srv.Client(), srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	jobs, err := s.Scrape(ctx, model.ScraperInput{ResultsWanted: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(jobs))
	}

	// First job
	j1 := jobs[0]
	if j1.ID != "echojobs-1001" {
		t.Fatalf("expected ID 'echojobs-1001', got %q", j1.ID)
	}
	if j1.Title != "AI Engineer" {
		t.Fatalf("expected title 'AI Engineer', got %q", j1.Title)
	}
	if j1.CompanyName != "Acme AI" {
		t.Fatalf("expected company 'Acme AI', got %q", j1.CompanyName)
	}
	if j1.CompanyLogo != "https://example.com/logo.png" {
		t.Fatalf("expected logo URL, got %q", j1.CompanyLogo)
	}
	if j1.JobURL != "https://example.com/jobs/1001" {
		t.Fatalf("expected job URL, got %q", j1.JobURL)
	}
	if j1.Description != "Build machine learning models" {
		t.Fatalf("expected description, got %q", j1.Description)
	}
	if j1.Location.City != "San Francisco, CA" {
		t.Fatalf("expected location 'San Francisco, CA', got %q", j1.Location.City)
	}
	if j1.Compensation == nil {
		t.Fatal("expected compensation, got nil")
	}
	if j1.Compensation.MinAmount == nil || *j1.Compensation.MinAmount != 120000 {
		t.Fatalf("expected min salary 120000, got %v", j1.Compensation.MinAmount)
	}
	if j1.Compensation.MaxAmount == nil || *j1.Compensation.MaxAmount != 180000 {
		t.Fatalf("expected max salary 180000, got %v", j1.Compensation.MaxAmount)
	}
	if j1.Compensation.Currency != "USD" {
		t.Fatalf("expected currency USD, got %q", j1.Compensation.Currency)
	}
	if j1.Compensation.Interval != model.IntervalYearly {
		t.Fatalf("expected yearly interval, got %q", j1.Compensation.Interval)
	}
	if j1.DatePosted == nil {
		t.Fatal("expected date_posted")
	}
	if !j1.DatePosted.Equal(time.Date(2026, 5, 20, 10, 0, 0, 0, time.UTC)) {
		t.Fatalf("expected 2026-05-20T10:00:00Z, got %v", j1.DatePosted)
	}
	if !j1.IsRemote {
		t.Fatal("expected remote = true")
	}
	if j1.JobType != "fulltime" {
		t.Fatalf("expected job_type 'fulltime', got %q", j1.JobType)
	}
	if len(j1.Skills) != 3 || j1.Skills[0] != "python" || j1.Skills[1] != "tensorflow" {
		t.Fatalf("unexpected skills: %v", j1.Skills)
	}

	// Second job — test alternative field names
	j2 := jobs[1]
	if j2.ID != "echojobs-2002" {
		t.Fatalf("expected ID 'echojobs-2002', got %q", j2.ID)
	}
	if j2.Title != "ML Engineer" {
		t.Fatalf("expected title 'ML Engineer', got %q", j2.Title)
	}
	if j2.CompanyName != "Brain Corp" {
		t.Fatalf("expected company 'Brain Corp', got %q", j2.CompanyName)
	}
	// Uses salary_min/max without currency — defaults to USD
	if j2.Compensation == nil {
		t.Fatal("expected compensation")
	}
	if j2.Compensation.Currency != "USD" {
		t.Fatalf("expected default USD, got %q", j2.Compensation.Currency)
	}
	// Uses published_at and remote (alternative field names)
	if j2.DatePosted == nil {
		t.Fatal("expected date from published_at")
	}
	if !j2.IsRemote {
		t.Fatal("expected remote from 'remote' field")
	}
}

func TestEchoJobsScraper_HandlesFlexibleResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		body := `{
			"jobs": [
				{
					"id": "flex1",
					"title": "Flexible Engineer",
					"company": "Flex Corp",
					"url": "https://flexcorp.com/jobs/flex1",
					"tags": ["go"]
				},
				{
					"id": "flex2",
					"title": "Resilient Developer",
					"company": "Resilient Inc",
					"url": "https://resilient.io/jobs/flex2"
				}
			]
		}`
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	s := sut.NewWithAPIURL(srv.Client(), srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	jobs, err := s.Scrape(ctx, model.ScraperInput{ResultsWanted: 10})
	if err != nil {
		t.Fatalf("unexpected error with flexible response: %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs from wrapper format, got %d", len(jobs))
	}
	if jobs[0].ID != "echojobs-flex1" {
		t.Fatalf("expected ID 'echojobs-flex1', got %q", jobs[0].ID)
	}
	if jobs[1].ID != "echojobs-flex2" {
		t.Fatalf("expected ID 'echojobs-flex2', got %q", jobs[1].ID)
	}
	if len(jobs[0].Skills) != 1 || jobs[0].Skills[0] != "go" {
		t.Fatalf("expected skills ['go'], got %v", jobs[0].Skills)
	}
}

func TestEchoJobsScraper_FiltersBySearchTerm(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		body := `[
			{"id": 1, "title": "AI Engineer", "company": "A", "url": "https://a.com/1"},
			{"id": 2, "title": "Backend Developer", "company": "B", "url": "https://b.com/2"},
			{"id": 3, "title": "ML Engineer", "company": "C", "url": "https://c.com/3"},
			{"id": 4, "title": "AI Product Manager", "company": "D", "url": "https://d.com/4"},
			{"id": 5, "title": "Data Scientist", "company": "E", "url": "https://e.com/5"}
		]`
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	s := sut.NewWithAPIURL(srv.Client(), srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	jobs, err := s.Scrape(ctx, model.ScraperInput{SearchTerm: "AI", ResultsWanted: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs matching 'AI', got %d", len(jobs))
	}
	if jobs[0].Title != "AI Engineer" {
		t.Fatalf("expected 'AI Engineer', got %q", jobs[0].Title)
	}
	if jobs[1].Title != "AI Product Manager" {
		t.Fatalf("expected 'AI Product Manager', got %q", jobs[1].Title)
	}
}

func TestEchoJobsScraper_RespectsResultsWanted(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		body := `[
			{"id": 1, "title": "Engineer A", "company": "A", "url": "https://a.com/1"},
			{"id": 2, "title": "Engineer B", "company": "B", "url": "https://b.com/2"},
			{"id": 3, "title": "Engineer C", "company": "C", "url": "https://c.com/3"},
			{"id": 4, "title": "Engineer D", "company": "D", "url": "https://d.com/4"},
			{"id": 5, "title": "Engineer E", "company": "E", "url": "https://e.com/5"}
		]`
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	s := sut.NewWithAPIURL(srv.Client(), srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	jobs, err := s.Scrape(ctx, model.ScraperInput{ResultsWanted: 3})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(jobs) != 3 {
		t.Fatalf("expected 3 jobs, got %d", len(jobs))
	}
}
