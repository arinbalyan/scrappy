package weworkremotely

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
  <title>We Work Remotely</title>
  <item>
    <title><![CDATA[TechCorp: Senior Go Developer]]></title>
    <link>https://weworkremotely.com/remote-jobs/techcorp-senior-go-developer</link>
    <guid>https://weworkremotely.com/remote-jobs/techcorp-senior-go-developer</guid>
    <description><![CDATA[Build scalable backend systems in Go.]]></description>
    <pubDate>Mon, 18 May 2026 10:00:00 GMT</pubDate>
    <region>Remote</region>
    <country>US</country>
    <skills>go, golang, backend</skills>
    <category>Engineering</category>
    <type>full-time</type>
  </item>
  <item>
    <title><![CDATA[WebInc: Frontend Developer]]></title>
    <link>https://weworkremotely.com/remote-jobs/webinc-frontend-developer</link>
    <guid>https://weworkremotely.com/remote-jobs/webinc-frontend-developer</guid>
    <description><![CDATA[React and TypeScript developer for remote team.]]></description>
    <pubDate>Tue, 19 May 2026 14:30:00 GMT</pubDate>
    <region>Europe</region>
    <skills>react, typescript, css</skills>
    <category>Engineering</category>
  </item>
  <item>
    <title></title>
    <link>https://weworkremotely.com/remote-jobs/empty-title</link>
  </item>
</channel>
</rss>`

func TestScraper_SiteName(t *testing.T) {
	s := New(nil)
	if got := s.SiteName(); got != model.SiteWeWorkRemotely {
		t.Errorf("SiteName() = %q, want %q", got, model.SiteWeWorkRemotely)
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

	// Job 0: Senior Go Developer
	j0 := jobs[0]
	if !stringsContains(j0.ID, "wwr-") {
		t.Errorf("job[0].ID should start with wwr-, got %q", j0.ID)
	}
	if j0.Title != "Senior Go Developer" {
		t.Errorf("job[0].Title = %q, want %q", j0.Title, "Senior Go Developer")
	}
	if j0.CompanyName != "TechCorp" {
		t.Errorf("job[0].CompanyName = %q, want %q", j0.CompanyName, "TechCorp")
	}
	if j0.Site != string(model.SiteWeWorkRemotely) {
		t.Errorf("job[0].Site = %q, want %q", j0.Site, model.SiteWeWorkRemotely)
	}
	if !j0.IsRemote {
		t.Error("job[0].IsRemote should be true")
	}
	if j0.DatePosted == nil {
		t.Error("job[0].DatePosted is nil")
	}
	if j0.Location.Country != "US" {
		t.Errorf("job[0].Location.Country = %q, want %q", j0.Location.Country, "US")
	}
	if j0.Location.City != "Remote" {
		t.Errorf("job[0].Location.City = %q, want %q", j0.Location.City, "Remote")
	}

	// Job 1: Frontend Developer
	j1 := jobs[1]
	if j1.Title != "Frontend Developer" {
		t.Errorf("job[1].Title = %q, want %q", j1.Title, "Frontend Developer")
	}
	if j1.CompanyName != "WebInc" {
		t.Errorf("job[1].CompanyName = %q, want %q", j1.CompanyName, "WebInc")
	}
	if j1.Location.City != "Europe" {
		t.Errorf("job[1].Location.City = %q, want %q", j1.Location.City, "Europe")
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
		SearchTerm:    "frontend",
		ResultsWanted: 25,
	})
	if err != nil {
		t.Fatalf("Scrape() returned error: %v", err)
	}

	if len(jobs) != 1 {
		t.Fatalf("expected 1 job matching 'frontend', got %d", len(jobs))
	}
	if jobs[0].Title != "Frontend Developer" {
		t.Errorf("job[0].Title = %q, want %q", jobs[0].Title, "Frontend Developer")
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

func stringsContains(s, substr string) bool {
	return len(s) >= len(substr) && containsStr(s, substr)
}

func containsStr(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
