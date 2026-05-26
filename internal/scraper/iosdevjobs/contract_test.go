package iosdevjobs

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
  <title>iOS Dev Jobs</title>
  <item>
    <title>Senior iOS Developer @ Apple</title>
    <link>https://iosdevjobs.com/job/senior-ios-developer-12345/</link>
    <guid>senior-ios-developer-12345</guid>
    <description><![CDATA[Build great iOS apps using Swift and SwiftUI.]]></description>
    <pubDate>Mon, 15 May 2026 10:00:00 GMT</pubDate>
  </item>
  <item>
    <title>iOS Engineer @ Google</title>
    <link>https://iosdevjobs.com/job/ios-engineer-67890/</link>
    <guid>ios-engineer-67890</guid>
    <description><![CDATA[Work on the next generation of iOS apps.]]></description>
    <pubDate>Tue, 16 May 2026 14:30:00 GMT</pubDate>
  </item>
  <item>
    <title>No Company Title</title>
    <link>https://iosdevjobs.com/job/no-company/</link>
    <guid>no-company</guid>
    <description>No company here</description>
    <pubDate>Wed, 17 May 2026 08:00:00 GMT</pubDate>
  </item>
  <item>
    <title></title>
    <link>https://iosdevjobs.com/job/empty-title/</link>
    <guid>empty-title</guid>
    <description>Empty title job</description>
    <pubDate>Thu, 18 May 2026 08:00:00 GMT</pubDate>
  </item>
</channel>
</rss>`

func TestScraper_SiteName(t *testing.T) {
	s := New(nil)
	if got := s.SiteName(); got != model.SiteIOSDevJobs {
		t.Errorf("SiteName() = %q, want %q", got, model.SiteIOSDevJobs)
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

	if len(jobs) != 3 {
		t.Fatalf("expected 3 jobs, got %d", len(jobs))
	}

	// Check first job
	j1 := jobs[0]
	if j1.ID != "iosdevjobs-senior-ios-developer-12345" {
		t.Errorf("job[0].ID = %q, want %q", j1.ID, "iosdevjobs-senior-ios-developer-12345")
	}
	if j1.Title != "Senior iOS Developer" {
		t.Errorf("job[0].Title = %q, want %q", j1.Title, "Senior iOS Developer")
	}
	if j1.CompanyName != "Apple" {
		t.Errorf("job[0].CompanyName = %q, want %q", j1.CompanyName, "Apple")
	}
	if j1.JobURL != "https://iosdevjobs.com/job/senior-ios-developer-12345/" {
		t.Errorf("job[0].JobURL = %q", j1.JobURL)
	}
	if j1.Site != string(model.SiteIOSDevJobs) {
		t.Errorf("job[0].Site = %q, want %q", j1.Site, model.SiteIOSDevJobs)
	}
	if j1.Description != "Build great iOS apps using Swift and SwiftUI." {
		t.Errorf("job[0].Description = %q", j1.Description)
	}
	if j1.DatePosted == nil {
		t.Error("job[0].DatePosted is nil, expected a parsed date")
	} else if !j1.DatePosted.Before(time.Now()) {
		t.Error("job[0].DatePosted should be in the past")
	}
	if j1.ApplyMethod != "external_url" {
		t.Errorf("job[0].ApplyMethod = %q, want %q", j1.ApplyMethod, "external_url")
	}

	// Check second job
	j2 := jobs[1]
	if j2.ID != "iosdevjobs-ios-engineer-67890" {
		t.Errorf("job[1].ID = %q, want %q", j2.ID, "iosdevjobs-ios-engineer-67890")
	}
	if j2.Title != "iOS Engineer" {
		t.Errorf("job[1].Title = %q, want %q", j2.Title, "iOS Engineer")
	}
	if j2.CompanyName != "Google" {
		t.Errorf("job[1].CompanyName = %q, want %q", j2.CompanyName, "Google")
	}

	// Check third job (no company)
	j3 := jobs[2]
	if j3.Title != "No Company Title" {
		t.Errorf("job[2].Title = %q, want %q", j3.Title, "No Company Title")
	}
	if j3.CompanyName != "" {
		t.Errorf("job[2].CompanyName = %q, want empty", j3.CompanyName)
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
		SearchTerm:    "Google",
		ResultsWanted: 25,
	})
	if err != nil {
		t.Fatalf("Scrape() returned error: %v", err)
	}

	if len(jobs) != 1 {
		t.Fatalf("expected 1 job matching 'Google', got %d", len(jobs))
	}
	if jobs[0].Title != "iOS Engineer" {
		t.Errorf("job[0].Title = %q, want %q", jobs[0].Title, "iOS Engineer")
	}
}

func TestScraper_FailsOnEmptyResponse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`<?xml version="1.0"?><rss version="2.0"><channel></channel></rss>`))
	}))
	defer ts.Close()

	s := NewWithFeedURL(nil, ts.URL)
	_, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 25})
	if err == nil {
		t.Fatal("expected error for empty response, got nil")
	}
}

func TestScraper_FailsOn429And503(t *testing.T) {
	tests := []struct {
		name string
		code int
	}{
		{"429 Too Many Requests", http.StatusTooManyRequests},
		{"503 Service Unavailable", http.StatusServiceUnavailable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.code)
			}))
			defer ts.Close()

			s := NewWithFeedURL(nil, ts.URL)
			_, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 25})
			if err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

func TestParseTitle(t *testing.T) {
	tests := []struct {
		input       string
		wantTitle   string
		wantCompany string
	}{
		{"Senior iOS Developer @ Apple", "Senior iOS Developer", "Apple"},
		{"iOS Engineer @ Google", "iOS Engineer", "Google"},
		{"No Company Title", "No Company Title", ""},
		{"Trailing @ ", "Trailing", ""},
	}
	for _, tt := range tests {
		jobTitle, company := parseTitle(tt.input)
		if jobTitle != tt.wantTitle {
			t.Errorf("parseTitle(%q) title = %q, want %q", tt.input, jobTitle, tt.wantTitle)
		}
		if company != tt.wantCompany {
			t.Errorf("parseTitle(%q) company = %q, want %q", tt.input, company, tt.wantCompany)
		}
	}
}
