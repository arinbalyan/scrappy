package eurojobs_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	sut "github.com/arinbalyan/scrappy/internal/scraper/eurojobs"
)

// sampleRSS is a minimal EuroJobs RSS feed with three job items used across
// happy-path tests. Matches the EuroJobs RSS feed structure at eurojobs.com/rss.
const sampleRSS = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
<channel>
<title>EuroJobs</title>
<link>https://www.eurojobs.com/</link>
<description>European job board</description>
<item>
<title>Software Engineer - Berlin</title>
<link>https://www.eurojobs.com/jobs/software-engineer-berlin-12345</link>
<guid>12345</guid>
<description><![CDATA[<p>We are looking for a Software Engineer in Berlin.</p><p>Experience with Go and Python required.</p>]]></description>
<pubDate>Tue, 21 May 2026 10:00:00 GMT</pubDate>
</item>
<item>
<title>AI Engineer - Madrid</title>
<link>https://www.eurojobs.com/jobs/ai-engineer-madrid-12346</link>
<guid>12346</guid>
<description><![CDATA[<p>AI Engineer position in Madrid working on ML models and NLP.</p>]]></description>
<pubDate>Mon, 20 May 2026 14:30:00 GMT</pubDate>
</item>
<item>
<title>DevOps Engineer - Vienna</title>
<link>https://www.eurojobs.com/jobs/devops-engineer-vienna-12347</link>
<guid>12347</guid>
<description><![CDATA[<p>Remote DevOps position with Kubernetes, AWS, and CI/CD pipelines.</p>]]></description>
<pubDate>Sun, 19 May 2026 08:00:00 GMT</pubDate>
</item>
</channel>
</rss>`

// TestEurojobsHappyPath verifies that a valid RSS response is parsed correctly
// into JobPost records with expected field mappings.
func TestEurojobsHappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(sampleRSS))
	}))
	defer srv.Close()

	s := sut.NewWithURLs(srv.Client(), srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	jobs, err := s.Scrape(ctx, model.ScraperInput{SearchTerm: "", ResultsWanted: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(jobs) != 3 {
		t.Fatalf("expected 3 jobs, got %d", len(jobs))
	}

	// Verify job 0: Software Engineer - Berlin
	if jobs[0].Title != "Software Engineer - Berlin" {
		t.Errorf("expected title 'Software Engineer - Berlin', got %q", jobs[0].Title)
	}
	if jobs[0].JobURL != "https://www.eurojobs.com/jobs/software-engineer-berlin-12345" {
		t.Errorf("unexpected URL: %q", jobs[0].JobURL)
	}
	if jobs[0].ID != "ej-12345" {
		t.Errorf("expected ID 'ej-12345', got %q", jobs[0].ID)
	}
	if jobs[0].DatePosted == nil {
		t.Error("expected DatePosted to be parsed")
	}
	if !stringsContains(jobs[0].Description, "Software Engineer") {
		t.Error("expected description to contain 'Software Engineer'")
	}

	// Verify job 1: AI Engineer - Madrid
	if jobs[1].Title != "AI Engineer - Madrid" {
		t.Errorf("expected title 'AI Engineer - Madrid', got %q", jobs[1].Title)
	}

	// Verify job 2: DevOps Engineer - Vienna
	if jobs[2].Title != "DevOps Engineer - Vienna" {
		t.Errorf("expected title 'DevOps Engineer - Vienna', got %q", jobs[2].Title)
	}
}

// TestEurojobsHandles429 verifies that a 429 Too Many Requests response
// propagates as an error.
func TestEurojobsHandles429(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	s := sut.NewWithURLs(srv.Client(), srv.URL)
	_, err := s.Scrape(context.Background(), model.ScraperInput{SearchTerm: "x", ResultsWanted: 1})
	if err == nil {
		t.Fatal("expected error for status 429")
	}
}

// TestEurojobsHandles503 verifies that a 503 Service Unavailable response
// propagates as an error.
func TestEurojobsHandles503(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	s := sut.NewWithURLs(srv.Client(), srv.URL)
	_, err := s.Scrape(context.Background(), model.ScraperInput{SearchTerm: "x", ResultsWanted: 1})
	if err == nil {
		t.Fatal("expected error for status 503")
	}
}

// TestEurojobsEmptyResponse verifies that an OK response with no RSS items
// returns an error (no meaningful jobs found).
func TestEurojobsEmptyResponse(t *testing.T) {
	emptyHTML := `<html><body>No jobs found</body></html>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(emptyHTML))
	}))
	defer srv.Close()

	s := sut.NewWithURLs(srv.Client(), srv.URL)
	_, err := s.Scrape(context.Background(), model.ScraperInput{SearchTerm: "x", ResultsWanted: 1})
	if err == nil {
		t.Fatal("expected error for empty RSS response")
	}
}

// TestEurojobsContextCancellation verifies that a context cancellation during
// the HTTP request is propagated as an error.
func TestEurojobsContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Delay longer than the context timeout to force cancellation.
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(sampleRSS))
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

// stringsContains is a helper to check substring presence.
func stringsContains(s, substr string) bool {
	return len(s) >= len(substr) && contains(s, substr)
}

// contains is a portable strings.Contains for Go.
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
