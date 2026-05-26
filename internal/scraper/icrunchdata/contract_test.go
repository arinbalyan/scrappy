package icrunchdata

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
  <title>iCrunchData</title>
  <item>
    <title>Data Scientist</title>
    <link>https://icrunchdata.com/job/data-scientist-12345/</link>
    <guid>https://icrunchdata.com/job/data-scientist-12345/</guid>
    <description><![CDATA[Build ML models and analyze large datasets.]]></description>
    <category>Data Science</category>
    <dc:creator>TechCorp</dc:creator>
    <pubDate>Mon, 15 May 2026 10:00:00 GMT</pubDate>
  </item>
  <item>
    <title>Data Analyst</title>
    <link>https://icrunchdata.com/job/data-analyst-67890/</link>
    <guid>https://icrunchdata.com/job/data-analyst-67890/</guid>
    <description><![CDATA[Analyze business data and create dashboards.]]></description>
    <category>Analytics</category>
    <dc:creator>Startup Inc</dc:creator>
    <pubDate>Tue, 16 May 2026 14:30:00 GMT</pubDate>
  </item>
  <item>
    <title></title>
    <link>https://icrunchdata.com/job/empty-title/</link>
    <guid>https://icrunchdata.com/job/empty-title/</guid>
    <description>Empty title job</description>
    <pubDate>Wed, 17 May 2026 08:00:00 GMT</pubDate>
  </item>
  <item>
    <title>No Link Job</title>
    <link></link>
    <guid></guid>
    <description>No link here</description>
    <pubDate>Thu, 18 May 2026 08:00:00 GMT</pubDate>
  </item>
</channel>
</rss>`

func TestScraper_SiteName(t *testing.T) {
	s := New(nil)
	if got := s.SiteName(); got != model.SiteIcrunchData {
		t.Errorf("SiteName() = %q, want %q", got, model.SiteIcrunchData)
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
	expectedID := "icrunchdata-data-scientist-12345"
	if j1.ID != expectedID {
		t.Errorf("job[0].ID = %q, want %q", j1.ID, expectedID)
	}
	if j1.Title != "Data Scientist" {
		t.Errorf("job[0].Title = %q, want %q", j1.Title, "Data Scientist")
	}
	if j1.JobURL != "https://icrunchdata.com/job/data-scientist-12345/" {
		t.Errorf("job[0].JobURL = %q", j1.JobURL)
	}
	if j1.Site != string(model.SiteIcrunchData) {
		t.Errorf("job[0].Site = %q, want %q", j1.Site, model.SiteIcrunchData)
	}
	if j1.CompanyName != "TechCorp" {
		t.Errorf("job[0].CompanyName = %q, want %q", j1.CompanyName, "TechCorp")
	}
	if j1.Description != "Build ML models and analyze large datasets." {
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
	expectedID2 := "icrunchdata-data-analyst-67890"
	if j2.ID != expectedID2 {
		t.Errorf("job[1].ID = %q, want %q", j2.ID, expectedID2)
	}
	if j2.Title != "Data Analyst" {
		t.Errorf("job[1].Title = %q, want %q", j2.Title, "Data Analyst")
	}
	if j2.CompanyName != "Startup Inc" {
		t.Errorf("job[1].CompanyName = %q, want %q", j2.CompanyName, "Startup Inc")
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
		SearchTerm:    "analyst",
		ResultsWanted: 25,
	})
	if err != nil {
		t.Fatalf("Scrape() returned error: %v", err)
	}

	if len(jobs) != 1 {
		t.Fatalf("expected 1 job matching 'analyst', got %d", len(jobs))
	}
	if jobs[0].Title != "Data Analyst" {
		t.Errorf("job[0].Title = %q, want %q", jobs[0].Title, "Data Analyst")
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

func TestNewWithFeedURL_EmptyEndpoint(t *testing.T) {
	s1 := NewWithFeedURL(nil, "")
	s2 := New(nil)
	if s1.feedURL != s2.feedURL {
		t.Errorf("empty endpoint should not override feed URL")
	}
}

func TestExtractTag_DcCreator(t *testing.T) {
	xml := `<dc:creator>TechCorp</dc:creator>`
	got := extractTag(xml, "dc:creator")
	if got != "TechCorp" {
		t.Errorf("extractTag(dc:creator) = %q, want %q", got, "TechCorp")
	}
}
