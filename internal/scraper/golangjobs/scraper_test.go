package golangjobs

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
)

const testRSS = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
<channel>
  <title>Golang Projects</title>
  <item>
    <title>Senior Go Developer</title>
    <link>https://www.golangprojects.com/golang-go-job-id/12345-senior-go-developer.html</link>
    <guid>https://www.golangprojects.com/golang-go-job-id/12345-senior-go-developer.html</guid>
    <description>&lt;p&gt;Build scalable backend systems in Go.&lt;/p&gt;</description>
    <category>Backend</category>
    <pubDate>Mon, 15 May 2026 10:00:00 GMT</pubDate>
  </item>
  <item>
    <title>Go Backend Engineer</title>
    <link>https://www.golangprojects.com/golang-go-job-id/67890-go-backend-engineer.html</link>
    <guid>https://www.golangprojects.com/golang-go-job-id/67890-go-backend-engineer.html</guid>
    <description><![CDATA[<p>Design and implement microservices in Go.</p>]]></description>
    <category>Backend</category>
    <pubDate>Tue, 16 May 2026 14:30:00 GMT</pubDate>
  </item>
  <item>
    <title></title>
    <link>https://www.golangprojects.com/golang-go-job-id/empty-title.html</link>
    <guid>https://www.golangprojects.com/golang-go-job-id/empty-title.html</guid>
    <description>Empty title job</description>
    <pubDate>Wed, 17 May 2026 08:00:00 GMT</pubDate>
  </item>
</channel>
</rss>`

func TestScraper_SiteName(t *testing.T) {
	s := New(nil)
	if got := s.SiteName(); got != model.SiteGolangJobs {
		t.Errorf("SiteName() = %q, want %q", got, model.SiteGolangJobs)
	}
}

func TestScraper_Scrape(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(testRSS))
	}))
	defer ts.Close()

	s := NewWithFeedURL(nil, ts.URL)
	jobs, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 25})
	if err != nil {
		t.Fatalf("Scrape() returned error: %v", err)
	}

	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(jobs))
	}

	// Check first job
	j1 := jobs[0]
	expectedID1 := "golangjobs-12345-senior-go-developer.html"
	if j1.ID != expectedID1 {
		t.Errorf("job[0].ID = %q, want %q", j1.ID, expectedID1)
	}
	if j1.Title != "Senior Go Developer" {
		t.Errorf("job[0].Title = %q, want %q", j1.Title, "Senior Go Developer")
	}
	if j1.JobURL != "https://www.golangprojects.com/golang-go-job-id/12345-senior-go-developer.html" {
		t.Errorf("job[0].JobURL = %q", j1.JobURL)
	}
	if j1.Site != string(model.SiteGolangJobs) {
		t.Errorf("job[0].Site = %q, want %q", j1.Site, model.SiteGolangJobs)
	}
	if j1.DatePosted == nil {
		t.Error("job[0].DatePosted is nil, expected a parsed date")
	} else if !j1.DatePosted.Before(time.Now()) {
		t.Error("job[0].DatePosted should be in the past")
	}

	// Check second job
	j2 := jobs[1]
	expectedID2 := "golangjobs-67890-go-backend-engineer.html"
	if j2.ID != expectedID2 {
		t.Errorf("job[1].ID = %q, want %q", j2.ID, expectedID2)
	}
	if j2.Title != "Go Backend Engineer" {
		t.Errorf("job[1].Title = %q, want %q", j2.Title, "Go Backend Engineer")
	}
}

func TestScraper_Scrape_SearchTerm(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(testRSS))
	}))
	defer ts.Close()

	s := NewWithFeedURL(nil, ts.URL)
	jobs, err := s.Scrape(context.Background(), model.ScraperInput{
		SearchTerm:    "backend",
		ResultsWanted: 25,
	})
	if err != nil {
		t.Fatalf("Scrape() returned error: %v", err)
	}

	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs matching 'backend', got %d", len(jobs))
	}

	// Both titles contain "backend"
	if jobs[0].Title != "Senior Go Developer" {
		t.Errorf("job[0].Title = %q, want %q", jobs[0].Title, "Senior Go Developer")
	}
	if jobs[1].Title != "Go Backend Engineer" {
		t.Errorf("job[1].Title = %q, want %q", jobs[1].Title, "Go Backend Engineer")
	}
}

func TestScraper_Scrape_HTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	s := NewWithFeedURL(nil, ts.URL)
	_, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 25})
	if err == nil {
		t.Fatal("expected error for 500 status, got nil")
	}
}

func TestScraper_Scrape_EmptyBody(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`<?xml version="1.0"?><rss version="2.0"><channel></channel></rss>`))
	}))
	defer ts.Close()

	s := NewWithFeedURL(nil, ts.URL)
	_, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 25})
	if err == nil {
		t.Fatal("expected error for empty feed, got nil")
	}
}

func TestNewWithFeedURL_EmptyEndpoint(t *testing.T) {
	s1 := NewWithFeedURL(nil, "")
	s2 := New(nil)
	if s1.feedURL != s2.feedURL {
		t.Errorf("empty endpoint should not override feed URL")
	}
}

func TestExtractID(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"https://www.golangprojects.com/golang-go-job-id/12345-senior-go-developer.html", "12345-senior-go-developer.html"},
		{"https://www.golangprojects.com/golang-go-job-id/some-slug", "some-slug"},
		{"", "unknown"},
	}
	for _, tt := range tests {
		got := extractID(tt.input)
		if got != tt.want {
			t.Errorf("extractID(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
