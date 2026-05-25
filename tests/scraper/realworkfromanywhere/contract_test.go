package realworkfromanywhere_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	sut "github.com/arinbalyan/scrappy/internal/scraper/realworkfromanywhere"
)

const sampleRSS = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
<channel>
<title>Real Work From Anywhere</title>
<item>
	<title><![CDATA[Senior Go Developer]]></title>
	<link><![CDATA[https://www.realworkfromanywhere.com/job/senior-go-developer-123]]></link>
	<guid><![CDATA[https://www.realworkfromanywhere.com/job/senior-go-developer-123]]></guid>
	<description><![CDATA[Build distributed systems in Go. Remote first team.]]></description>
	<pubDate><![CDATA[Mon, 20 May 2026 10:00:00 +0000]]></pubDate>
	<category>Engineering</category>
</item>
<item>
	<title><![CDATA[DevOps Engineer]]></title>
	<link><![CDATA[https://www.realworkfromanywhere.com/job/devops-engineer-456]]></link>
	<guid><![CDATA[https://www.realworkfromanywhere.com/job/devops-engineer-456]]></guid>
	<description><![CDATA[Manage cloud infrastructure.]]></description>
	<pubDate><![CDATA[Tue, 19 May 2026 08:00:00 +0000]]></pubDate>
	<category>DevOps</category>
</item>
</channel>
</rss>`

func TestRealWorkFromAnywhereFailsOnEmptyResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<?xml version="1.0"?><rss><channel></channel></rss>`))
	}))
	defer srv.Close()

	s := sut.NewWithAPIURL(srv.Client(), srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := s.Scrape(ctx, model.ScraperInput{SearchTerm: "engineer", ResultsWanted: 5})
	if err == nil {
		t.Fatal("expected error for empty upstream response (no items)")
	}
}

func TestRealWorkFromAnywhereFailsOn429And503(t *testing.T) {
	for _, status := range []int{http.StatusTooManyRequests, http.StatusServiceUnavailable} {
		t.Run(fmt.Sprintf("status_%d", status), func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(status)
			}))
			defer srv.Close()

			s := sut.NewWithAPIURL(srv.Client(), srv.URL)
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

func TestRealWorkFromAnywhereScraper_ScrapeBasic(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(sampleRSS))
	}))
	defer srv.Close()

	s := sut.NewWithAPIURL(srv.Client(), srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	jobs, err := s.Scrape(ctx, model.ScraperInput{
		SearchTerm:    "",
		ResultsWanted: 10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(jobs))
	}

	// --- First job ---
	j1 := jobs[0]
	if !strings.HasPrefix(j1.ID, "rwfa-") {
		t.Fatalf("expected ID to start with 'rwfa-', got %q", j1.ID)
	}
	if j1.Title != "Senior Go Developer" {
		t.Fatalf("expected title 'Senior Go Developer', got %q", j1.Title)
	}
	if !strings.Contains(j1.JobURL, "senior-go-developer-123") {
		t.Fatalf("expected job URL to contain 'senior-go-developer-123', got %q", j1.JobURL)
	}
	if !j1.IsRemote {
		t.Fatal("expected IsRemote = true")
	}
	if j1.Description == "" {
		t.Fatal("expected description")
	}
	if j1.DatePosted == nil {
		t.Fatal("expected date_posted to be set")
	}
	if !j1.DatePosted.Equal(time.Date(2026, 5, 20, 10, 0, 0, 0, time.FixedZone("", 0))) {
		t.Fatalf("expected 2026-05-20 10:00:00 UTC, got %v", j1.DatePosted)
	}

	// --- Second job ---
	j2 := jobs[1]
	if j2.Title != "DevOps Engineer" {
		t.Fatalf("expected title 'DevOps Engineer', got %q", j2.Title)
	}
	if !j2.IsRemote {
		t.Fatal("expected IsRemote = true")
	}
	if j2.DatePosted != nil {
		expected := time.Date(2026, 5, 19, 8, 0, 0, 0, time.FixedZone("", 0))
		if !j2.DatePosted.Equal(expected) {
			t.Fatalf("expected 2026-05-19 08:00:00 UTC, got %v", j2.DatePosted)
		}
	}
}

func TestRealWorkFromAnywhereScraper_SearchFilter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(sampleRSS))
	}))
	defer srv.Close()

	s := sut.NewWithAPIURL(srv.Client(), srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Search for "devops" — should only match the second job
	jobs, err := s.Scrape(ctx, model.ScraperInput{
		SearchTerm:    "devops",
		ResultsWanted: 10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job (search filter 'devops'), got %d", len(jobs))
	}
	if !strings.Contains(jobs[0].Title, "DevOps") {
		t.Fatalf("expected DevOps job, got %q", jobs[0].Title)
	}
}

func TestRealWorkFromAnywhereScraper_PlainContentRSS(t *testing.T) {
	// Test with non-CDATA content
	plainRSS := `<?xml version="1.0"?>
<rss><channel>
<item>
	<title>Full Stack Developer</title>
	<link>https://www.realworkfromanywhere.com/job/fullstack-789</link>
	<guid>https://www.realworkfromanywhere.com/job/fullstack-789</guid>
	<description>Build frontend and backend systems.</description>
	<pubDate>Wed, 21 May 2026 12:00:00 +0000</pubDate>
	<category>Engineering</category>
</item>
</channel></rss>`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(plainRSS))
	}))
	defer srv.Close()

	s := sut.NewWithAPIURL(srv.Client(), srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	jobs, err := s.Scrape(ctx, model.ScraperInput{
		SearchTerm:    "",
		ResultsWanted: 10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
	if jobs[0].Title != "Full Stack Developer" {
		t.Fatalf("expected title 'Full Stack Developer', got %q", jobs[0].Title)
	}
}
