package virtualvocations

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/arinbalyan/scrappy/internal/model"
)

const testRSS = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
<channel>
  <title>Virtual Vocations</title>
  <item>
    <title><![CDATA[Remote Software Engineer]]></title>
    <link>https://www.virtualvocations.com/jobs/remote-software-engineer/</link>
    <guid>https://www.virtualvocations.com/jobs/remote-software-engineer/</guid>
    <description><![CDATA[Build great software from anywhere.]]></description>
    <pubDate>Mon, 18 May 2026 10:00:00 GMT</pubDate>
  </item>
  <item>
    <title><![CDATA[Virtual Customer Service Rep]]></title>
    <link>https://www.virtualvocations.com/jobs/customer-service-rep/</link>
    <guid>https://www.virtualvocations.com/jobs/customer-service-rep/</guid>
    <description><![CDATA[Help customers from home.]]></description>
    <pubDate>Tue, 19 May 2026 14:30:00 GMT</pubDate>
  </item>
  <item>
    <title></title>
    <link>https://www.virtualvocations.com/jobs/empty-title/</link>
    <description>Empty title item</description>
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
		w.Header().Set("Content-Type", "application/rss+xml")
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
	if j0.ID != "virtualvocations-remote-software-engineer" {
		t.Errorf("job[0].ID = %q, want %q", j0.ID, "virtualvocations-remote-software-engineer")
	}
	if j0.Title != "Remote Software Engineer" {
		t.Errorf("job[0].Title = %q, want %q", j0.Title, "Remote Software Engineer")
	}
	if j0.Site != string(model.SiteVirtualVocations) {
		t.Errorf("job[0].Site = %q, want %q", j0.Site, model.SiteVirtualVocations)
	}
	if !j0.IsRemote {
		t.Error("job[0].IsRemote should be true for VirtualVocations")
	}
	if j0.DatePosted == nil {
		t.Error("job[0].DatePosted is nil")
	}

	// Job 1
	j1 := jobs[1]
	if j1.Title != "Virtual Customer Service Rep" {
		t.Errorf("job[1].Title = %q, want %q", j1.Title, "Virtual Customer Service Rep")
	}
	if j1.DatePosted == nil {
		t.Error("job[1].DatePosted is nil")
	}
}

func TestScraper_Scrape_SearchTerm(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(testRSS))
	}))
	defer ts.Close()

	s := NewWithRSSURL(nil, ts.URL)
	jobs, err := s.Scrape(context.Background(), model.ScraperInput{
		SearchTerm:    "Customer",
		ResultsWanted: 25,
	})
	if err != nil {
		t.Fatalf("Scrape() returned error: %v", err)
	}

	if len(jobs) != 1 {
		t.Fatalf("expected 1 job matching 'Customer', got %d", len(jobs))
	}
	if jobs[0].Title != "Virtual Customer Service Rep" {
		t.Errorf("job[0].Title = %q, want %q", jobs[0].Title, "Virtual Customer Service Rep")
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

func TestScraper_Scrape_EmptyBody(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, `<?xml version="1.0"?><rss><channel></channel></rss>`)
	}))
	defer ts.Close()

	s := NewWithRSSURL(nil, ts.URL)
	_, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 25})
	if err == nil {
		t.Fatal("expected error for empty response, got nil")
	}
}

func TestNewWithRSSURL_EmptyEndpoint(t *testing.T) {
	s1 := NewWithRSSURL(nil, "")
	s2 := New(nil)
	if s1.rssURL != s2.rssURL {
		t.Errorf("empty endpoint should not override RSS URL")
	}
}
