// Package upwork_test holds contract tests for the Upwork scraper.
//
// Test pattern follows tests/scraper/google/contract_test.go:
//   - HappyPath:          server returns valid HTML with JSON-LD; verify 3 jobs parsed
//   - ErrorHandling429:   server returns 429; verify error
//   - ErrorHandling503:   server returns 503; verify error
//   - EmptyResponse:      server returns empty HTML; verify error
//   - ContextCancellation: context cancelled before scrape; verify error
package upwork_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	sut "github.com/arinbalyan/scrappy/internal/scraper/upwork"
)

// mockUpworkHTML returns a representative Upwork search results page with
// JSON-LD structured data for 3 job listings.
func mockUpworkHTML() string {
	return `<!DOCTYPE html><html lang="en"><head>
<meta charset="utf-8"><title>Upwork Job Search</title>
<script type="application/ld+json">
{
    "@context": "https://schema.org",
    "@type": "ItemList",
    "itemListElement": [
        {
            "@type": "ListItem",
            "position": 1,
            "item": {
                "@type": "JobPosting",
                "title": "Go Developer",
                "url": "https://www.upwork.com/jobs/~abc123/",
                "description": "Looking for an experienced Go developer for backend services and microservices architecture. Must know Go, PostgreSQL, and gRPC.",
                "datePosted": "2024-06-15T10:30:00Z",
                "hiringOrganization": {
                    "@type": "Organization",
                    "name": "TechClient Inc"
                }
            }
        },
        {
            "@type": "ListItem",
            "position": 2,
            "item": {
                "@type": "JobPosting",
                "title": "Full Stack Engineer",
                "url": "https://www.upwork.com/jobs/~def456/",
                "description": "Full stack developer with React and Node.js for a 6-month project building a SaaS platform.",
                "datePosted": "2024-06-14T08:00:00Z",
                "hiringOrganization": {
                    "@type": "Organization",
                    "name": "StartupXYZ"
                }
            }
        },
        {
            "@type": "ListItem",
            "position": 3,
            "item": {
                "@type": "JobPosting",
                "title": "Machine Learning Engineer",
                "url": "https://www.upwork.com/jobs/~ghi789/",
                "description": "ML engineer needed for NLP text classification project using Transformers and PyTorch.",
                "datePosted": "2024-06-13T12:00:00Z",
                "hiringOrganization": {
                    "@type": "Organization",
                    "name": "AIData Corp"
                }
            }
        }
    ]
}
</script></head><body><div class="job-search-results"></div></body></html>`
}

// mockEmptyHTML returns a minimal HTML page with no job data.
func mockEmptyHTML() string {
	return `<html><head></head><body><div class="no-results">No jobs found</div></body></html>`
}

// TestUpworkHappyPath verifies that a realistic Upwork search page with
// JSON-LD structured data is correctly parsed into 3 JobPost records.
func TestUpworkHappyPath(t *testing.T) {
	html := mockUpworkHTML()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(html))
	}))
	defer srv.Close()

	s := sut.NewWithURLs(srv.Client(), srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	jobs, err := s.Scrape(ctx, model.ScraperInput{
		SearchTerm:    "go developer",
		ResultsWanted: 3,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(jobs) != 3 {
		t.Fatalf("expected 3 jobs, got %d", len(jobs))
	}

	// Verify first job fields
	if jobs[0].Title != "Go Developer" {
		t.Errorf("expected title 'Go Developer', got '%s'", jobs[0].Title)
	}
	if jobs[0].CompanyName != "TechClient Inc" {
		t.Errorf("expected company 'TechClient Inc', got '%s'", jobs[0].CompanyName)
	}
	if jobs[0].JobURL != "https://www.upwork.com/jobs/~abc123/" {
		t.Errorf("unexpected job URL: %s", jobs[0].JobURL)
	}
	if jobs[0].ID == "" {
		t.Error("expected non-empty job ID")
	}
	if !stringsHasPrefix(jobs[0].ID, "up-") {
		t.Errorf("expected job ID to start with 'up-', got '%s'", jobs[0].ID)
	}

	// Verify second job
	if jobs[1].Title != "Full Stack Engineer" {
		t.Errorf("expected title 'Full Stack Engineer', got '%s'", jobs[1].Title)
	}
	if jobs[1].CompanyName != "StartupXYZ" {
		t.Errorf("expected company 'StartupXYZ', got '%s'", jobs[1].CompanyName)
	}

	// Verify third job
	if jobs[2].Title != "Machine Learning Engineer" {
		t.Errorf("expected title 'Machine Learning Engineer', got '%s'", jobs[2].Title)
	}
	if jobs[2].CompanyName != "AIData Corp" {
		t.Errorf("expected company 'AIData Corp', got '%s'", jobs[2].CompanyName)
	}
}

// TestUpworkErrorHandling429 verifies that a 429 Too Many Requests response
// causes the scraper to return an error.
func TestUpworkErrorHandling429(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	s := sut.NewWithURLs(srv.Client(), srv.URL)
	_, err := s.Scrape(context.Background(), model.ScraperInput{
		SearchTerm:    "x",
		ResultsWanted: 1,
	})
	if err == nil {
		t.Fatal("expected error for 429 status")
	}
}

// TestUpworkErrorHandling503 verifies that a 503 Service Unavailable response
// causes the scraper to return an error.
func TestUpworkErrorHandling503(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	s := sut.NewWithURLs(srv.Client(), srv.URL)
	_, err := s.Scrape(context.Background(), model.ScraperInput{
		SearchTerm:    "x",
		ResultsWanted: 1,
	})
	if err == nil {
		t.Fatal("expected error for 503 status")
	}
}

// TestUpworkEmptyResponse verifies that an HTML page with no job listings
// causes the scraper to return an error via HasMeaningfulJobs check.
func TestUpworkEmptyResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(mockEmptyHTML()))
	}))
	defer srv.Close()

	s := sut.NewWithURLs(srv.Client(), srv.URL)
	_, err := s.Scrape(context.Background(), model.ScraperInput{
		SearchTerm:    "x",
		ResultsWanted: 1,
	})
	if err == nil {
		t.Fatal("expected error for empty response")
	}
}

// TestUpworkContextCancellation verifies that a cancelled context causes
// the scraper to stop and return an error immediately.
func TestUpworkContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Should not be reached, but serve OK just in case
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<html><body></body></html>`))
	}))
	defer srv.Close()

	s := sut.NewWithURLs(srv.Client(), srv.URL)
	_, err := s.Scrape(ctx, model.ScraperInput{
		SearchTerm:    "x",
		ResultsWanted: 1,
	})
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

// stringsHasPrefix is a helper to check string prefix without importing
// a full strings package reference in test assertions.
func stringsHasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
