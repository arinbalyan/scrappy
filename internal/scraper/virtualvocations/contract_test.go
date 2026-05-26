package virtualvocations

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
)

const testRSS = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
<channel>
<item>
  <title><![CDATA[Remote Software Engineer]]></title>
  <link>https://www.virtualvocations.com/jobs/remote-software-engineer-12345</link>
  <guid>https://www.virtualvocations.com/jobs/remote-software-engineer-12345</guid>
  <description><![CDATA[Build great software from anywhere.]]></description>
  <pubDate>Mon, 20 May 2026 10:00:00 +0000</pubDate>
</item>
<item>
  <title>Data Analyst (Remote)</title>
  <link>https://www.virtualvocations.com/jobs/data-analyst-67890</link>
  <guid>https://www.virtualvocations.com/jobs/data-analyst-67890</guid>
  <description>Analyze data for clients.</description>
  <pubDate>Tue, 19 May 2026 08:30:00 +0000</pubDate>
</item>
<item>
  <title></title>
  <link></link>
  <guid></guid>
  <description></description>
  <pubDate></pubDate>
</item>
</channel>
</rss>`

func TestScraper_SiteName(t *testing.T) {
	s := New(nil)
	if got := s.SiteName(); got != model.SiteVirtualVocations {
		t.Errorf("SiteName() = %q, want %q", got, model.SiteVirtualVocations)
	}
}

func TestScraper_Scrape(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(testRSS))
	}))
	defer ts.Close()

	s := NewWithRSSURL(nil, ts.URL)
	jobs, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 25})
	if err != nil {
		t.Fatalf("Scrape() returned error: %v", err)
	}

	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(jobs))
	}

	// Job 0
	j0 := jobs[0]
	if j0.Title != "Remote Software Engineer" {
		t.Errorf("job[0].Title = %q, want %q", j0.Title, "Remote Software Engineer")
	}
	if j0.Site != string(model.SiteVirtualVocations) {
		t.Errorf("job[0].Site = %q, want %q", j0.Site, model.SiteVirtualVocations)
	}
	if !strings.Contains(j0.JobURL, "remote-software-engineer-12345") {
		t.Errorf("job[0].JobURL = %q, should contain job ID", j0.JobURL)
	}
	if !j0.IsRemote {
		t.Error("job[0].IsRemote = false, want true")
	}
	if j0.DatePosted == nil {
		t.Error("job[0].DatePosted is nil, expected a parsed date")
	}
	if j0.Description != "Build great software from anywhere." {
		t.Errorf("job[0].Description = %q", j0.Description)
	}

	// Job 1
	j1 := jobs[1]
	if j1.Title != "Data Analyst (Remote)" {
		t.Errorf("job[1].Title = %q, want %q", j1.Title, "Data Analyst (Remote)")
	}
	if !j1.IsRemote {
		t.Error("job[1].IsRemote = false, want true")
	}
	if j1.DatePosted == nil {
		t.Error("job[1].DatePosted is nil")
	}
}

func TestScraper_Scrape_SearchFilter(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(testRSS))
	}))
	defer ts.Close()

	s := NewWithRSSURL(nil, ts.URL)
	jobs, err := s.Scrape(context.Background(), model.ScraperInput{
		ResultsWanted: 25,
		SearchTerm:    "Data Analyst",
	})
	if err != nil {
		t.Fatalf("Scrape() returned error: %v", err)
	}

	if len(jobs) != 1 {
		t.Fatalf("expected 1 job matching 'Data Analyst', got %d", len(jobs))
	}
	if jobs[0].Title != "Data Analyst (Remote)" {
		t.Errorf("job.Title = %q, want %q", jobs[0].Title, "Data Analyst (Remote)")
	}
}

func TestScraper_Scrape_HTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	s := NewWithRSSURL(nil, ts.URL)
	_, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 25})
	if err == nil {
		t.Fatal("expected error for 500 status, got nil")
	}
}

func TestScraper_Scrape_EmptyResponse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`<?xml version="1.0"?><rss><channel></channel></rss>`))
	}))
	defer ts.Close()

	s := NewWithRSSURL(nil, ts.URL)
	_, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 25})
	if err == nil {
		t.Fatal("expected error for empty response, got nil")
	}
}

func TestScraper_Scrape_429(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer ts.Close()

	s := NewWithRSSURL(nil, ts.URL)
	_, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 25})
	if err == nil {
		t.Fatal("expected error for 429 status, got nil")
	}
}

func TestDatePosted(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(testRSS))
	}))
	defer ts.Close()

	s := NewWithRSSURL(nil, ts.URL)
	jobs, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 25})
	if err != nil {
		t.Fatalf("Scrape() returned error: %v", err)
	}

	if len(jobs) > 0 && jobs[0].DatePosted != nil {
		// Verify the date parsed correctly (Mon, 20 May 2026 10:00:00 +0000)
		expected := time.Date(2026, 5, 20, 10, 0, 0, 0, time.UTC)
		if !jobs[0].DatePosted.Equal(expected) {
			t.Errorf("job[0].DatePosted = %v, want %v", jobs[0].DatePosted, expected)
		}
	}
}

func TestNewWithRSSURL_EmptyEndpoint(t *testing.T) {
	s1 := NewWithRSSURL(nil, "")
	s2 := New(nil)
	if s1.rssURL != s2.rssURL {
		t.Errorf("empty endpoint should not override RSS URL")
	}
}
