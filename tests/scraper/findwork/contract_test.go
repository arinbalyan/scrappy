package findwork_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	sut "github.com/arinbalyan/scrappy/internal/scraper/findwork"
)

func TestFindworkHappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify auth header is set
		if r.Header.Get("Authorization") != "Token test-key-123" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"count": 2,
			"next": null,
			"previous": null,
			"results": [
				{
					"id": 1001,
					"role": "AI Engineer",
					"company_name": "OpenAI",
					"company_num_employees": null,
					"employment_type": "full_time",
					"location": "San Francisco, CA",
					"remote": false,
					"logo": null,
					"url": "https://openai.com/careers/ai-engineer",
					"text": "<p>Build the future of AI.</p>",
					"date_posted": "2025-01-15",
					"keywords": ["AI", "Machine Learning", "Python"],
					"source": "findwork"
				},
				{
					"id": 1002,
					"role": "ML Engineer",
					"company_name": "Google",
					"company_num_employees": "10000+",
					"employment_type": null,
					"location": "Mountain View, CA",
					"remote": true,
					"logo": null,
					"url": "https://google.com/careers/ml-engineer",
					"text": null,
					"date_posted": "2025-01-14",
					"keywords": ["ML", "TensorFlow"],
					"source": "findwork"
				}
			]
		}`))
	}))
	defer srv.Close()

	s := sut.NewWithURLs(srv.Client(), srv.URL, "test-key-123")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	jobs, err := s.Scrape(ctx, model.ScraperInput{
		SearchTerm:    "AI Engineer",
		ResultsWanted: 10,
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(jobs))
	}

	// Verify first job
	job0 := jobs[0]
	if job0.ID != "findwork-1001" {
		t.Errorf("expected ID findwork-1001, got %s", job0.ID)
	}
	if job0.Title != "AI Engineer" {
		t.Errorf("expected Title 'AI Engineer', got %s", job0.Title)
	}
	if job0.CompanyName != "OpenAI" {
		t.Errorf("expected CompanyName 'OpenAI', got %s", job0.CompanyName)
	}
	if job0.JobURL != "https://openai.com/careers/ai-engineer" {
		t.Errorf("unexpected JobURL: %s", job0.JobURL)
	}
	if job0.Location.City != "San Francisco, CA" {
		t.Errorf("expected Location 'San Francisco, CA', got %s", job0.Location.City)
	}
	if job0.IsRemote {
		t.Error("expected IsRemote false for job 0")
	}
	if !strings.Contains(job0.Description, "Build the future of AI") {
		t.Errorf("expected description to contain 'Build the future of AI', got %s", job0.Description)
	}
	if job0.DatePosted == nil {
		t.Fatal("expected DatePosted to be set")
	}
	expectedDate := time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC)
	if !job0.DatePosted.Equal(expectedDate) {
		t.Errorf("expected DatePosted 2025-01-15, got %v", *job0.DatePosted)
	}
	if len(job0.Skills) != 3 {
		t.Errorf("expected 3 skills, got %d: %v", len(job0.Skills), job0.Skills)
	}

	// Verify second job
	job1 := jobs[1]
	if job1.ID != "findwork-1002" {
		t.Errorf("expected ID findwork-1002, got %s", job1.ID)
	}
	if job1.Title != "ML Engineer" {
		t.Errorf("expected Title 'ML Engineer', got %s", job1.Title)
	}
	if !job1.IsRemote {
		t.Error("expected IsRemote true for job 1")
	}
}

func TestFindworkErrorHandling(t *testing.T) {
	tests := []struct {
		name   string
		status int
	}{
		{"Status429", http.StatusTooManyRequests},
		{"Status503", http.StatusServiceUnavailable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
			}))
			defer srv.Close()

			s := sut.NewWithURLs(srv.Client(), srv.URL, "test-key")
			_, err := s.Scrape(context.Background(), model.ScraperInput{
				SearchTerm:    "engineer",
				ResultsWanted: 1,
			})
			if err == nil {
				t.Fatalf("expected error for status %d", tt.status)
			}
			if !strings.Contains(err.Error(), fmt.Sprintf("%d", tt.status)) {
				t.Errorf("expected error to contain status code %d, got %v", tt.status, err)
			}
		})
	}
}

func TestFindworkEmptyResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"count":0,"next":null,"previous":null,"results":[]}`))
	}))
	defer srv.Close()

	s := sut.NewWithURLs(srv.Client(), srv.URL, "test-key")
	_, err := s.Scrape(context.Background(), model.ScraperInput{
		SearchTerm:    "nonexistent",
		ResultsWanted: 10,
	})
	if err == nil {
		t.Fatal("expected error for empty response")
	}
}

func TestFindworkContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate slow response
		time.Sleep(500 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"count":1,"results":[{"id":1,"role":"Late Job","url":"https://example.com/1"}]}`))
	}))
	defer srv.Close()

	s := sut.NewWithURLs(srv.Client(), srv.URL, "test-key")
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	// Give the context time to expire before calling.
	time.Sleep(10 * time.Millisecond)

	_, err := s.Scrape(ctx, model.ScraperInput{
		SearchTerm:    "test",
		ResultsWanted: 5,
	})
	if err == nil {
		t.Fatal("expected context deadline exceeded error")
	}
}

func TestFindworkPagination(t *testing.T) {
	pageCalls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pageCalls++
		if pageCalls == 1 {
			// First page: return 1 job with next URL
			nextURL := r.URL.Scheme + "://" + r.URL.Host + "/?page=2"
			_, _ = w.Write([]byte(`{
				"count": 2,
				"next": "` + nextURL + `",
				"previous": null,
				"results": [
					{"id":1,"role":"Job One","company_name":"Co A","url":"https://a.com/1","date_posted":"2025-01-01","keywords":[]}
				]
			}`))
		} else {
			// Second page: return another job, no next
			_, _ = w.Write([]byte(`{
				"count": 2,
				"next": null,
				"previous": null,
				"results": [
					{"id":2,"role":"Job Two","company_name":"Co B","url":"https://b.com/2","date_posted":"2025-01-02","keywords":[]}
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
}
