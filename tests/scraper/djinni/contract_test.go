package djinni_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	sut "github.com/arinbalyan/scrappy/internal/scraper/djinni"
)

// sampleRSS is a minimal Djinni RSS feed with three job items used across
// happy-path tests.
const sampleRSS = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
<channel>
<title>Djinni</title>
<link>https://djinni.co/</link>
<description>Jobs for tech professionals</description>
<item>
<title>Senior Software Engineer</title>
<link>https://djinni.co/jobs/12345-senior-software-engineer/</link>
<guid>12345</guid>
<description><![CDATA[<p>We are hiring a Senior Software Engineer with Go experience.</p>]]></description>
<pubDate>Tue, 21 May 2026 10:00:00 GMT</pubDate>
<category>Engineering</category>
</item>
<item>
<title>AI Engineer</title>
<link>https://djinni.co/jobs/12346-ai-engineer/</link>
<guid>12346</guid>
<description><![CDATA[<p>Looking for an AI Engineer to work on ML models.</p>]]></description>
<pubDate>Mon, 20 May 2026 14:30:00 GMT</pubDate>
<category>AI/ML</category>
</item>
<item>
<title>DevOps Engineer (Remote)</title>
<link>https://djinni.co/jobs/12347-devops-engineer/</link>
<guid>12347</guid>
<description><![CDATA[<p>Remote DevOps position with Kubernetes and AWS.</p>]]></description>
<pubDate>Sun, 19 May 2026 08:00:00 GMT</pubDate>
<category>DevOps</category>
</item>
</channel>
</rss>`

// TestDjinniHappyPath verifies that a valid RSS response is parsed correctly
// into JobPost records with expected field mappings.
func TestDjinniHappyPath(t *testing.T) {
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

	// Verify job 0: Senior Software Engineer
	if jobs[0].Title != "Senior Software Engineer" {
		t.Errorf("expected title 'Senior Software Engineer', got %q", jobs[0].Title)
	}
	if jobs[0].JobURL != "https://djinni.co/jobs/12345-senior-software-engineer/" {
		t.Errorf("unexpected URL: %q", jobs[0].JobURL)
	}
	if jobs[0].ID != "dj-12345" {
		t.Errorf("expected ID 'dj-12345', got %q", jobs[0].ID)
	}
	if jobs[0].DatePosted == nil {
		t.Error("expected DatePosted to be parsed")
	}
	if jobs[0].IsRemote {
		t.Error("expected IsRemote=false for non-remote job")
	}

	// Verify job 2: DevOps Engineer (Remote)
	if jobs[2].Title != "DevOps Engineer (Remote)" {
		t.Errorf("expected title 'DevOps Engineer (Remote)', got %q", jobs[2].Title)
	}
	if !jobs[2].IsRemote {
		t.Error("expected IsRemote=true for remote job")
	}
}

// TestDjinniHandles429 verifies that a 429 Too Many Requests response
// propagates as an error.
func TestDjinniHandles429(t *testing.T) {
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

// TestDjinniHandles503 verifies that a 503 Service Unavailable response
// propagates as an error.
func TestDjinniHandles503(t *testing.T) {
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

// TestDjinniEmptyResponse verifies that an OK response with no RSS items
// returns an error (no meaningful jobs found).
func TestDjinniEmptyResponse(t *testing.T) {
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

// TestDjinniContextCancellation verifies that a context timeout during the
// HTTP request is propagated as an error.
func TestDjinniContextCancellation(t *testing.T) {
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
