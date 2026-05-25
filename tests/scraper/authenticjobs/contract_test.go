package authenticjobs_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	sut "github.com/arinbalyan/scrappy/internal/scraper/authenticjobs"
)

func TestAuthenticJobsFailsOnEmptyResponse(t *testing.T) {
	t.Setenv("AUTHENTICJOBS_API_KEY", "test-key")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"listings":[]}`))
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

func TestAuthenticJobsFailsOn429And503(t *testing.T) {
	t.Setenv("AUTHENTICJOBS_API_KEY", "test-key")
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

func TestAuthenticJobsScraper_ScrapeBasic(t *testing.T) {
	t.Setenv("AUTHENTICJOBS_API_KEY", "test-key")
	pageCalls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pageCalls++
		// Return 2 jobs only on the first page
		if pageCalls == 1 {
			if r.URL.Query().Get("api_key") != "test-key" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
			"listings": [
				{
					"id": "12345",
					"title": "Software Engineer",
					"company": {"name": "TechCorp", "url": "https://techcorp.com"},
					"description": "Build amazing software",
					"perks": "Flexible hours, remote work",
					"howto_apply": "Apply via our website",
					"post_date": "2026-05-20",
					"telecommuting": "yes",
					"location": {"name": "Remote"}
				},
				{
					"id": "67890",
					"title": "Backend Developer",
					"company": {"name": "DevInc", "url": ""},
					"description": "Backend systems",
					"perks": "",
					"howto_apply": "",
					"post_date": "2026-05-19",
					"telecommuting": "no",
					"location": {"name": "San Francisco, CA"}
				}
			]
		}`))
		} else {
			// Second page returns empty to stop pagination
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"listings":[]}`))
		}
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
	if j1.ID != "authenticjobs-12345" {
		t.Fatalf("expected ID 'authenticjobs-12345', got %q", j1.ID)
	}
	if j1.Title != "Software Engineer" {
		t.Fatalf("expected title 'Software Engineer', got %q", j1.Title)
	}
	if j1.CompanyName != "TechCorp" {
		t.Fatalf("expected company 'TechCorp', got %q", j1.CompanyName)
	}
	if j1.CompanyURL != "https://techcorp.com" {
		t.Fatalf("expected company URL, got %q", j1.CompanyURL)
	}
	if j1.JobURL != "https://authenticjobs.com/jobs/12345" {
		t.Fatalf("expected job URL, got %q", j1.JobURL)
	}
	if j1.Description == "" {
		t.Fatal("expected description")
	}
	if !j1.IsRemote {
		t.Fatal("expected IsRemote = true for first job")
	}
	if j1.Location.City != "Remote" {
		t.Fatalf("expected city 'Remote', got %q", j1.Location.City)
	}
	if j1.DatePosted == nil {
		t.Fatal("expected date_posted to be set")
	}
	if !j1.DatePosted.Equal(time.Date(2026, 5, 20, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("expected 2026-05-20, got %v", j1.DatePosted)
	}

	// --- Second job: no remote, has location ---
	j2 := jobs[1]
	if j2.ID != "authenticjobs-67890" {
		t.Fatalf("expected ID 'authenticjobs-67890', got %q", j2.ID)
	}
	if j2.Title != "Backend Developer" {
		t.Fatalf("expected title 'Backend Developer', got %q", j2.Title)
	}
	if j2.CompanyName != "DevInc" {
		t.Fatalf("expected company 'DevInc', got %q", j2.CompanyName)
	}
	if j2.IsRemote {
		t.Fatal("expected IsRemote = false for second job")
	}
	if j2.Location.City != "San Francisco, CA" {
		t.Fatalf("expected city 'San Francisco, CA', got %q", j2.Location.City)
	}
	if j2.DatePosted == nil {
		t.Fatal("expected date_posted for second job")
	}
	if !j2.DatePosted.Equal(time.Date(2026, 5, 19, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("expected 2026-05-19, got %v", j2.DatePosted)
	}
}

func TestAuthenticJobsScraper_SearchQueryInURL(t *testing.T) {
	t.Setenv("AUTHENTICJOBS_API_KEY", "test-key")
	var capturedKeywords string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedKeywords = r.URL.Query().Get("keywords")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"listings":[]}`))
	}))
	defer srv.Close()

	s := sut.NewWithURLs(srv.Client(), srv.URL, "test-key")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, _ = s.Scrape(ctx, model.ScraperInput{
		SearchTerm:    "software engineer",
		ResultsWanted: 5,
	})
	if capturedKeywords != "software engineer" {
		t.Fatalf("expected keywords='software engineer', got %q", capturedKeywords)
	}
}

func TestAuthenticJobsScraper_MissingAPIKey(t *testing.T) {
	s := sut.NewWithURLs(nil, "https://example.com", "")
	_, err := s.Scrape(context.Background(), model.ScraperInput{
		SearchTerm:    "engineer",
		ResultsWanted: 5,
	})
	if err == nil {
		t.Fatal("expected error when API key is empty")
	}
	if !strings.Contains(err.Error(), "AUTHENTICJOBS_API_KEY not set") {
		t.Fatalf("expected error to mention missing key, got %v", err)
	}
}

func TestAuthenticJobsScraper_Pagination(t *testing.T) {
	t.Setenv("AUTHENTICJOBS_API_KEY", "test-key")
	pageCalls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pageCalls++
		w.WriteHeader(http.StatusOK)
		if pageCalls == 1 {
			// First page: return 1 hit
			_, _ = w.Write([]byte(`{
				"listings": [
					{
						"id": "page1job",
						"title": "First Page Job",
						"company": {"name": "Co A", "url": ""},
						"post_date": "2026-01-01"
					}
				]
			}`))
		} else if pageCalls == 2 {
			// Second page: return the second hit
			_, _ = w.Write([]byte(`{
				"listings": [
					{
						"id": "page2job",
						"title": "Second Page Job",
						"company": {"name": "Co B", "url": ""},
						"post_date": "2026-01-02"
					}
				]
			}`))
		} else {
			// Third page: empty to stop pagination
			_, _ = w.Write([]byte(`{"listings":[]}`))
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
	if pageCalls < 2 {
		t.Fatalf("expected at least 2 page calls, got %d", pageCalls)
	}
	if jobs[0].ID != "authenticjobs-page1job" {
		t.Fatalf("expected first job ID 'authenticjobs-page1job', got %q", jobs[0].ID)
	}
	if jobs[1].ID != "authenticjobs-page2job" {
		t.Fatalf("expected second job ID 'authenticjobs-page2job', got %q", jobs[1].ID)
	}
}