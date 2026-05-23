package iosdevjobs_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	sut "github.com/arinbalyan/scrappy/internal/scraper/iosdevjobs"
)

func TestIOSDevJobsParsesJobs(t *testing.T) {
	rss := `<?xml version="1.0"?><rss><channel><item><title><![CDATA[Senior Go Engineer @ Acme]]></title><link>https://iosdevjobs.com/jobs/senior-go-engineer-acme</link><description><![CDATA[Build cool APIs]]></description><pubDate>Mon, 20 May 2025 12:00:00 +0000</pubDate></item><item><title><![CDATA[iOS Developer @ Startup]]></title><link>https://iosdevjobs.com/jobs/ios-developer</link><description><![CDATA[Swift, UIKit]]></description><pubDate>Tue, 21 May 2025 10:00:00 +0000</pubDate></item></channel></rss>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(rss))
	}))
	defer srv.Close()
	s := sut.NewWithURLs(srv.Client(), srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	jobs, err := s.Scrape(ctx, model.ScraperInput{SearchTerm: "", ResultsWanted: 5})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(jobs))
	}
	if jobs[0].Title != "Senior Go Engineer" {
		t.Fatalf("expected title 'Senior Go Engineer', got %q", jobs[0].Title)
	}
	if jobs[0].CompanyName != "Acme" {
		t.Fatalf("expected company 'Acme', got %q", jobs[0].CompanyName)
	}
	if !strings.Contains(jobs[0].Description, "Build cool APIs") {
		t.Fatalf("expected description to mention APIs, got %q", jobs[0].Description)
	}
	if jobs[0].DatePosted == nil {
		t.Fatal("expected DatePosted to be set")
	}
}

func TestIOSDevJobsFailsOn429And503(t *testing.T) {
	for _, status := range []int{http.StatusTooManyRequests, http.StatusServiceUnavailable} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(status)
		}))
		s := sut.NewWithURLs(srv.Client(), srv.URL)
		_, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 5})
		srv.Close()
		if err == nil {
			t.Fatalf("expected error for status %d", status)
		}
	}
}

func TestIOSDevJobsFailsOnEmptyFeed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<?xml version="1.0"?><rss><channel></channel></rss>`))
	}))
	defer srv.Close()
	s := sut.NewWithURLs(srv.Client(), srv.URL)
	_, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 5})
	if err == nil {
		t.Fatal("expected error for empty RSS feed")
	}
}
