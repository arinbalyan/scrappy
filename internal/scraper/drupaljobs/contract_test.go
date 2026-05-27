package drupaljobs

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
  <title>Drupal Jobs</title>
  <item>
    <title>Senior Drupal Developer</title>
    <link>https://www.drupal.org/jobs/senior-drupal-developer</link>
    <guid>https://www.drupal.org/jobs/senior-drupal-developer</guid>
    <description>&lt;p&gt;Build and maintain Drupal sites.&lt;/p&gt;</description>
    <category>Engineering</category>
    <pubDate>Mon, 15 May 2026 10:00:00 GMT</pubDate>
  </item>
  <item>
    <title>Drupal Site Builder</title>
    <link>https://www.drupal.org/jobs/drupal-site-builder</link>
    <guid>https://www.drupal.org/jobs/drupal-site-builder</guid>
    <description><![CDATA[<p>Create Drupal sites with modern tools.</p>]]></description>
    <category>Engineering</category>
    <pubDate>Tue, 16 May 2026 14:30:00 GMT</pubDate>
  </item>
  <item>
    <title></title>
    <link>https://www.drupal.org/jobs/empty-title</link>
    <guid>https://www.drupal.org/jobs/empty-title</guid>
    <description>Empty title job</description>
    <pubDate>Wed, 17 May 2026 08:00:00 GMT</pubDate>
  </item>
</channel>
</rss>`

func TestScraper_SiteName(t *testing.T) {
	s := New(nil)
	if got := s.SiteName(); got != model.SiteDrupalJobs {
		t.Errorf("SiteName() = %q, want %q", got, model.SiteDrupalJobs)
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
	j0 := jobs[0]
	if j0.ID != "drupaljobs-senior-drupal-developer" {
		t.Errorf("job[0].ID = %q, want %q", j0.ID, "drupaljobs-senior-drupal-developer")
	}
	if j0.Title != "Senior Drupal Developer" {
		t.Errorf("job[0].Title = %q, want %q", j0.Title, "Senior Drupal Developer")
	}
	if j0.JobURL != "https://www.drupal.org/jobs/senior-drupal-developer" {
		t.Errorf("job[0].JobURL = %q", j0.JobURL)
	}
	if j0.Site != string(model.SiteDrupalJobs) {
		t.Errorf("job[0].Site = %q", j0.Site)
	}
	if j0.DatePosted == nil {
		t.Error("job[0].DatePosted is nil")
	} else if !j0.DatePosted.Before(time.Now()) {
		t.Error("job[0].DatePosted should be in the past")
	}

	// Check second job
	j1 := jobs[1]
	if j1.ID != "drupaljobs-drupal-site-builder" {
		t.Errorf("job[1].ID = %q", j1.ID)
	}
	if j1.Title != "Drupal Site Builder" {
		t.Errorf("job[1].Title = %q", j1.Title)
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
		SearchTerm:    "builder",
		ResultsWanted: 25,
	})
	if err != nil {
		t.Fatalf("Scrape() returned error: %v", err)
	}

	if len(jobs) != 1 {
		t.Fatalf("expected 1 job matching 'builder', got %d", len(jobs))
	}
	if jobs[0].Title != "Drupal Site Builder" {
		t.Errorf("job[0].Title = %q, want %q", jobs[0].Title, "Drupal Site Builder")
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
