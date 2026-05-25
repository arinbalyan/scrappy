package jobtechdev_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	sut "github.com/arinbalyan/scrappy/internal/scraper/jobtechdev"
)

func TestJobTechDevFailsOnEmptyResponse(t *testing.T) {
	t.Setenv("JOBTECHDEV_API_KEY", "test-key")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"total":{"value":0},"hits":[]}`))
	}))
	defer srv.Close()

	s := sut.NewWithURLs(srv.Client(), srv.URL, "test-key")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := s.Scrape(ctx, model.ScraperInput{SearchTerm: "software engineer", ResultsWanted: 5})
	if err == nil {
		t.Fatal("expected error for empty upstream response")
	}
}

func TestJobTechDevFailsOn429And503(t *testing.T) {
	t.Setenv("JOBTECHDEV_API_KEY", "test-key")
	for _, status := range []int{http.StatusTooManyRequests, http.StatusServiceUnavailable} {
		t.Run(fmt.Sprintf("status_%d", status), func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(status)
			}))
			defer srv.Close()

			s := sut.NewWithURLs(srv.Client(), srv.URL, "test-key")
			_, err := s.Scrape(context.Background(), model.ScraperInput{
				SearchTerm:    "engineer",
				ResultsWanted: 1,
			})
			if err == nil {
				t.Fatalf("expected error for status %d", status)
			}
			if !strings.Contains(err.Error(), fmt.Sprintf("%d", status)) {
				t.Errorf("expected error to contain status %d, got %v", status, err)
			}
		})
	}
}

func TestJobTechDevScraper_ScrapeBasic(t *testing.T) {
	t.Setenv("JOBTECHDEV_API_KEY", "test-key")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the api-key header is sent.
		if r.Header.Get("api-key") != "test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"total": {"value": 2},
			"hits": [
				{
					"id": "abc123",
					"headline": "Software Engineer",
					"description": {"text": "Build amazing software"},
					"employment_type": {"label": "fulltime"},
					"working_hours_type": {"label": "office hours"},
					"employer": {"name": "TechCorp", "url": "https://techcorp.se"},
					"workplace_address": {
						"municipality": "Stockholm",
						"region": "Stockholm County",
						"country": "Sweden"
					},
					"application_details": {"url": "https://techcorp.se/apply", "email": null},
					"publication_date": "2026-05-20T10:00:00Z",
					"last_publication_date": "2026-06-20T10:00:00Z",
					"webpage_url": "https://techcorp.se/jobs/abc123",
					"logo_url": "https://techcorp.se/logo.png",
					"salary_description": "Competitive",
					"scope_of_work": {"min": 40, "max": 40}
				},
				{
					"id": "def456",
					"headline": "Backend Developer",
					"description": {"text": "Backend systems"},
					"employment_type": {"label": "parttime"},
					"working_hours_type": null,
					"employer": {"name": "DevInc", "url": null},
					"workplace_address": {
						"municipality": "Gothenburg",
						"region": "Vastra Gotaland",
						"country": "Sweden"
					},
					"application_details": {"url": "https://devinc.se/apply", "email": "hr@devinc.se"},
					"publication_date": "2026-05-19T08:30:00Z",
					"last_publication_date": null,
					"webpage_url": null,
					"logo_url": null,
					"salary_description": null,
					"scope_of_work": null
				}
			]
		}`))
	}))
	defer srv.Close()

	s := sut.NewWithURLs(srv.Client(), srv.URL, "test-key")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	jobs, err := s.Scrape(ctx, model.ScraperInput{
		SearchTerm:    "software engineer",
		ResultsWanted: 10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(jobs))
	}

	// --- First job: full fields ---
	j1 := jobs[0]
	if j1.ID != "jobtechdev-abc123" {
		t.Fatalf("expected ID 'jobtechdev-abc123', got %q", j1.ID)
	}
	if j1.Title != "Software Engineer" {
		t.Fatalf("expected title 'Software Engineer', got %q", j1.Title)
	}
	if j1.CompanyName != "TechCorp" {
		t.Fatalf("expected company 'TechCorp', got %q", j1.CompanyName)
	}
	if j1.CompanyLogo != "https://techcorp.se/logo.png" {
		t.Fatalf("expected logo URL, got %q", j1.CompanyLogo)
	}
	if j1.JobURL != "https://techcorp.se/jobs/abc123" {
		t.Fatalf("expected job URL 'https://techcorp.se/jobs/abc123', got %q", j1.JobURL)
	}
	if j1.Description != "Build amazing software" {
		t.Fatalf("expected description, got %q", j1.Description)
	}
	if j1.Location.City != "Stockholm" {
		t.Fatalf("expected city 'Stockholm', got %q", j1.Location.City)
	}
	if j1.Location.State != "Stockholm County" {
		t.Fatalf("expected state 'Stockholm County', got %q", j1.Location.State)
	}
	if j1.Location.Country != "Sweden" {
		t.Fatalf("expected country 'Sweden', got %q", j1.Location.Country)
	}
	if j1.IsRemote {
		t.Fatal("expected IsRemote = false")
	}
	if j1.DatePosted == nil {
		t.Fatal("expected date_posted to be set")
	}
	if !j1.DatePosted.Equal(time.Date(2026, 5, 20, 10, 0, 0, 0, time.UTC)) {
		t.Fatalf("expected 2026-05-20T10:00:00Z, got %v", j1.DatePosted)
	}
	if j1.JobType != "fulltime" {
		t.Fatalf("expected job_type 'fulltime', got %q", j1.JobType)
	}

	// --- Second job: no webpage_url, no logo — tests fallback paths ---
	j2 := jobs[1]
	if j2.ID != "jobtechdev-def456" {
		t.Fatalf("expected ID 'jobtechdev-def456', got %q", j2.ID)
	}
	if j2.Title != "Backend Developer" {
		t.Fatalf("expected title 'Backend Developer', got %q", j2.Title)
	}
	if j2.CompanyName != "DevInc" {
		t.Fatalf("expected company 'DevInc', got %q", j2.CompanyName)
	}
	// Falls back to application_details.url when webpage_url is null.
	if j2.JobURL != "https://devinc.se/apply" {
		t.Fatalf("expected fallback job URL 'https://devinc.se/apply', got %q", j2.JobURL)
	}
	if j2.CompanyLogo != "" {
		t.Fatalf("expected empty logo for second job, got %q", j2.CompanyLogo)
	}
	if j2.Location.City != "Gothenburg" {
		t.Fatalf("expected city 'Gothenburg', got %q", j2.Location.City)
	}
	if j2.Location.State != "Vastra Gotaland" {
		t.Fatalf("expected state 'Vastra Gotaland', got %q", j2.Location.State)
	}
	if j2.Location.Country != "Sweden" {
		t.Fatalf("expected country 'Sweden', got %q", j2.Location.Country)
	}
	if j2.IsRemote {
		t.Fatal("expected IsRemote = false for second job")
	}
	if j2.DatePosted == nil {
		t.Fatal("expected date_posted for second job")
	}
	if !j2.DatePosted.Equal(time.Date(2026, 5, 19, 8, 30, 0, 0, time.UTC)) {
		t.Fatalf("expected 2026-05-19T08:30:00Z, got %v", j2.DatePosted)
	}
	if j2.JobType != "parttime" {
		t.Fatalf("expected job_type 'parttime', got %q", j2.JobType)
	}
}

func TestJobTechDevScraper_SearchQueryInURL(t *testing.T) {
	t.Setenv("JOBTECHDEV_API_KEY", "test-key")
	var capturedQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedQuery = r.URL.Query().Get("q")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"total":{"value":0},"hits":[]}`))
	}))
	defer srv.Close()

	s := sut.NewWithURLs(srv.Client(), srv.URL, "test-key")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, _ = s.Scrape(ctx, model.ScraperInput{
		SearchTerm:    "software engineer",
		ResultsWanted: 5,
	})
	if capturedQuery != "software engineer" {
		t.Fatalf("expected q='software engineer', got %q", capturedQuery)
	}
}

func TestJobTechDevScraper_RespectsMaxLimit(t *testing.T) {
	t.Setenv("JOBTECHDEV_API_KEY", "test-key")
	var capturedLimit string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedLimit = r.URL.Query().Get("limit")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"total":{"value":0},"hits":[]}`))
	}))
	defer srv.Close()

	s := sut.NewWithURLs(srv.Client(), srv.URL, "test-key")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, _ = s.Scrape(ctx, model.ScraperInput{
		SearchTerm:    "go",
		ResultsWanted: 999,
	})
	if capturedLimit != "100" {
		t.Fatalf("expected limit=100 (maxLimit) when requesting 999, got %q", capturedLimit)
	}
}

func TestJobTechDevScraper_MissingAPIKey(t *testing.T) {
	s := sut.NewWithURLs(nil, "https://example.com", "")
	_, err := s.Scrape(context.Background(), model.ScraperInput{
		SearchTerm:    "engineer",
		ResultsWanted: 5,
	})
	if err == nil {
		t.Fatal("expected error when API key is empty")
	}
	if !strings.Contains(err.Error(), "JOBTECHDEV_API_KEY not set") {
		t.Fatalf("expected error to mention missing key, got %v", err)
	}
}

func TestJobTechDevScraper_Pagination(t *testing.T) {
	t.Setenv("JOBTECHDEV_API_KEY", "test-key")
	pageCalls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pageCalls++
		offset := r.URL.Query().Get("offset")
		_ = offset
		if pageCalls == 1 {
			// First page: return 1 hit, total=2
			_, _ = w.Write([]byte(`{
				"total": {"value": 2},
				"hits": [
					{
						"id": "page1job",
						"headline": "First Page Job",
						"employer": {"name": "Co A"},
						"publication_date": "2026-01-01T00:00:00Z"
					}
				]
			}`))
		} else {
			// Second page: return the second hit
			_, _ = w.Write([]byte(`{
				"total": {"value": 2},
				"hits": [
					{
						"id": "page2job",
						"headline": "Second Page Job",
						"employer": {"name": "Co B"},
						"publication_date": "2026-01-02T00:00:00Z"
					}
				]
			}`))
		}
	}))
	defer srv.Close()

	s := sut.NewWithURLs(srv.Client(), srv.URL, "test-key")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	jobs, err := s.Scrape(ctx, model.ScraperInput{
		SearchTerm:    "test",
		ResultsWanted: 10,
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs from pagination, got %d", len(jobs))
	}
	if pageCalls != 2 {
		t.Fatalf("expected 2 page calls, got %d", pageCalls)
	}
	if jobs[0].ID != "jobtechdev-page1job" {
		t.Fatalf("expected first job ID 'jobtechdev-page1job', got %q", jobs[0].ID)
	}
	if jobs[1].ID != "jobtechdev-page2job" {
		t.Fatalf("expected second job ID 'jobtechdev-page2job', got %q", jobs[1].ID)
	}
}
