package coroflot

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
  <title>Coroflot Design Jobs</title>
  <item>
    <title>Acme Design Co is seeking a Senior UX Designer</title>
    <link>https://www.coroflot.com/jobs/senior-ux-designer/12345</link>
    <guid>https://www.coroflot.com/jobs/senior-ux-designer/12345</guid>
    <description>&lt;p&gt;Lead UX design for our platform. Location: San Francisco, CA.&lt;/p&gt;</description>
    <pubDate>Mon, 15 May 2026 10:00:00 GMT</pubDate>
  </item>
  <item>
    <title>Creative Studio is seeking a Graphic Designer</title>
    <link>https://www.coroflot.com/jobs/graphic-designer/67890</link>
    <guid>https://www.coroflot.com/jobs/graphic-designer/67890</guid>
    <description><![CDATA[<p>Create stunning visuals. Based in New York, NY.</p>]]></description>
    <pubDate>Tue, 16 May 2026 14:30:00 GMT</pubDate>
  </item>
  <item>
    <title></title>
    <link>https://www.coroflot.com/jobs/empty-title/</link>
    <guid>https://www.coroflot.com/jobs/empty-title/</guid>
    <description>Empty title job</description>
    <pubDate>Wed, 17 May 2026 08:00:00 GMT</pubDate>
  </item>
</channel>
</rss>`

func TestScraper_SiteName(t *testing.T) {
	s := New(nil)
	if got := s.SiteName(); got != model.SiteCoroflot {
		t.Errorf("SiteName() = %q, want %q", got, model.SiteCoroflot)
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
	if j0.ID != "coroflot-12345" {
		t.Errorf("job[0].ID = %q, want %q", j0.ID, "coroflot-12345")
	}
	if j0.Title != "Acme Design Co is seeking a Senior UX Designer" {
		t.Errorf("job[0].Title = %q", j0.Title)
	}
	if j0.CompanyName != "Acme Design Co" {
		t.Errorf("job[0].CompanyName = %q, want %q", j0.CompanyName, "Acme Design Co")
	}
	if j0.Location.City != "San Francisco, CA" {
		t.Errorf("job[0].Location.City = %q, want %q", j0.Location.City, "San Francisco, CA")
	}
	if j0.Site != string(model.SiteCoroflot) {
		t.Errorf("job[0].Site = %q", j0.Site)
	}
	if j0.DatePosted == nil {
		t.Error("job[0].DatePosted is nil")
	} else if !j0.DatePosted.Before(time.Now()) {
		t.Error("job[0].DatePosted should be in the past")
	}

	// Check second job
	j1 := jobs[1]
	if j1.CompanyName != "Creative Studio" {
		t.Errorf("job[1].CompanyName = %q, want %q", j1.CompanyName, "Creative Studio")
	}
	if j1.Title != "Creative Studio is seeking a Graphic Designer" {
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
		SearchTerm:    "graphic",
		ResultsWanted: 25,
	})
	if err != nil {
		t.Fatalf("Scrape() returned error: %v", err)
	}

	if len(jobs) != 1 {
		t.Fatalf("expected 1 job matching 'graphic', got %d", len(jobs))
	}
	if jobs[0].Title != "Creative Studio is seeking a Graphic Designer" {
		t.Errorf("job[0].Title = %q", jobs[0].Title)
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
