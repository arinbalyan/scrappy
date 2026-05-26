package opensourcedesignjobs

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/arinbalyan/scrappy/internal/model"
)

const testRSS = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
<channel>
  <title>Open Source Design Jobs</title>
  <item>
    <title>UI/UX Designer</title>
    <link>https://opensourcedesign.net/jobs/ui-ux-designer/</link>
    <guid>https://opensourcedesign.net/jobs/ui-ux-designer/</guid>
    <description>&lt;p&gt;Design user interfaces for open source projects.&lt;/p&gt;</description>
    <pubDate>Mon, 15 May 2026 10:00:00 GMT</pubDate>
  </item>
  <item>
    <title>Branding Specialist</title>
    <link>https://opensourcedesign.net/jobs/branding-specialist/</link>
    <guid>https://opensourcedesign.net/jobs/branding-specialist/</guid>
    <description><![CDATA[<p>Work on visual identity for open source tools.</p>]]></description>
    <pubDate>Tue, 16 May 2026 14:30:00 GMT</pubDate>
  </item>
  <item>
    <title></title>
    <link>https://opensourcedesign.net/jobs/empty-title/</link>
    <guid>https://opensourcedesign.net/jobs/empty-title/</guid>
    <description>Empty title job</description>
    <pubDate>Wed, 17 May 2026 08:00:00 GMT</pubDate>
  </item>
</channel>
</rss>`

func TestScraper_SiteName(t *testing.T) {
	s := New(nil)
	if got := s.SiteName(); got != model.SiteOpenSourceDesignJobs {
		t.Errorf("SiteName() = %q, want %q", got, model.SiteOpenSourceDesignJobs)
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

	j1 := jobs[0]
	if j1.ID != "opensourcedesignjobs-ui-ux-designer" {
		t.Errorf("job[0].ID = %q, want %q", j1.ID, "opensourcedesignjobs-ui-ux-designer")
	}
	if j1.Title != "UI/UX Designer" {
		t.Errorf("job[0].Title = %q, want %q", j1.Title, "UI/UX Designer")
	}
	if j1.JobURL != "https://opensourcedesign.net/jobs/ui-ux-designer/" {
		t.Errorf("job[0].JobURL = %q", j1.JobURL)
	}
	if j1.Site != string(model.SiteOpenSourceDesignJobs) {
		t.Errorf("job[0].Site = %q, want %q", j1.Site, model.SiteOpenSourceDesignJobs)
	}

	j2 := jobs[1]
	if j2.ID != "opensourcedesignjobs-branding-specialist" {
		t.Errorf("job[1].ID = %q, want %q", j2.ID, "opensourcedesignjobs-branding-specialist")
	}
	if j2.Title != "Branding Specialist" {
		t.Errorf("job[1].Title = %q, want %q", j2.Title, "Branding Specialist")
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
		SearchTerm:    "branding",
		ResultsWanted: 25,
	})
	if err != nil {
		t.Fatalf("Scrape() returned error: %v", err)
	}

	if len(jobs) != 1 {
		t.Fatalf("expected 1 job matching 'branding', got %d", len(jobs))
	}
	if jobs[0].Title != "Branding Specialist" {
		t.Errorf("job[0].Title = %q, want %q", jobs[0].Title, "Branding Specialist")
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
