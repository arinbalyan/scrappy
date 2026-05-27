package wordpressjobs

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
  <title>WordPress Jobs</title>
  <item>
    <title>WordPress Developer Needed</title>
    <link>https://jobs.wordpress.net/job/wordpress-developer-needed/</link>
    <guid>https://jobs.wordpress.net/?p=12345</guid>
    <description>Looking for an experienced WordPress developer.</description>
    <pubDate>Mon, 18 May 2026 10:00:00 GMT</pubDate>
    <category>Development</category>
  </item>
  <item>
    <title>Freelance WordPress Designer</title>
    <link>https://jobs.wordpress.net/job/freelance-designer/</link>
    <guid>https://jobs.wordpress.net/?p=67890</guid>
    <description>Need a skilled designer for WordPress themes.</description>
    <pubDate>Tue, 19 May 2026 14:30:00 GMT</pubDate>
    <category>Design</category>
  </item>
  <item>
    <title></title>
    <link>https://jobs.wordpress.net/job/empty-title/</link>
  </item>
</channel>
</rss>`

func TestScraper_SiteName(t *testing.T) {
	s := New(nil)
	if got := s.SiteName(); got != model.SiteWordPressJobs {
		t.Errorf("SiteName() = %q, want %q", got, model.SiteWordPressJobs)
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
	if j0.ID != "wpjobs-12345" {
		t.Errorf("job[0].ID = %q, want %q", j0.ID, "wpjobs-12345")
	}
	if j0.Title != "WordPress Developer Needed" {
		t.Errorf("job[0].Title = %q, want %q", j0.Title, "WordPress Developer Needed")
	}
	if j0.Site != string(model.SiteWordPressJobs) {
		t.Errorf("job[0].Site = %q, want %q", j0.Site, model.SiteWordPressJobs)
	}
	if j0.DatePosted == nil {
		t.Error("job[0].DatePosted is nil")
	}

	// Job 1
	j1 := jobs[1]
	if j1.ID != "wpjobs-67890" {
		t.Errorf("job[1].ID = %q, want %q", j1.ID, "wpjobs-67890")
	}
	if j1.Title != "Freelance WordPress Designer" {
		t.Errorf("job[1].Title = %q, want %q", j1.Title, "Freelance WordPress Designer")
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
		SearchTerm:    "Designer",
		ResultsWanted: 25,
	})
	if err != nil {
		t.Fatalf("Scrape() returned error: %v", err)
	}

	if len(jobs) != 1 {
		t.Fatalf("expected 1 job matching 'Designer', got %d", len(jobs))
	}
	if jobs[0].Title != "Freelance WordPress Designer" {
		t.Errorf("job[0].Title = %q, want %q", jobs[0].Title, "Freelance WordPress Designer")
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
