package greenjobsboard

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
  <title>Green Jobs Board</title>
  <item>
    <title>Sustainability Manager</title>
    <link>https://greenjobs.greenjobsearch.org/job/sustainability-manager/</link>
    <guid>https://greenjobs.greenjobsearch.org/job/sustainability-manager/</guid>
    <description>&lt;p&gt;Lead sustainability initiatives.&lt;/p&gt;</description>
    <pubDate>Mon, 15 May 2026 10:00:00 GMT</pubDate>
    <dc:creator>GreenEarth Inc.</dc:creator>
    <content:encoded>&lt;p&gt;Full description for sustainability role.&lt;/p&gt;</content:encoded>
  </item>
  <item>
    <title>Environmental Officer</title>
    <link>https://greenjobs.greenjobsearch.org/job/env-officer/</link>
    <guid>https://greenjobs.greenjobsearch.org/job/env-officer/</guid>
    <description>Short description</description>
    <pubDate>Tue, 16 May 2026 14:30:00 GMT</pubDate>
    <dc:creator>EcoCorp</dc:creator>
  </item>
  <item>
    <title></title>
    <link>https://greenjobs.greenjobsearch.org/job/empty/</link>
    <guid>https://greenjobs.greenjobsearch.org/job/empty/</guid>
    <description>Empty</description>
    <pubDate>Wed, 17 May 2026 08:00:00 GMT</pubDate>
  </item>
</channel>
</rss>`

func TestScraper_SiteName(t *testing.T) {
	s := New(nil)
	if got := s.SiteName(); got != model.SiteGreenJobsBoard {
		t.Errorf("SiteName() = %q, want %q", got, model.SiteGreenJobsBoard)
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

	// First job should have content:encoded as description and dc:creator as company
	j1 := jobs[0]
	if j1.ID != "greenjobsboard-sustainability-manager" {
		t.Errorf("job[0].ID = %q, want %q", j1.ID, "greenjobsboard-sustainability-manager")
	}
	if j1.Title != "Sustainability Manager" {
		t.Errorf("job[0].Title = %q, want %q", j1.Title, "Sustainability Manager")
	}
	if j1.CompanyName != "GreenEarth Inc." {
		t.Errorf("job[0].CompanyName = %q, want %q", j1.CompanyName, "GreenEarth Inc.")
	}
	if j1.Description != "&lt;p&gt;Full description for sustainability role.&lt;/p&gt;" {
		t.Errorf("job[0].Description = %q", j1.Description)
	}
	if j1.Site != string(model.SiteGreenJobsBoard) {
		t.Errorf("job[0].Site = %q, want %q", j1.Site, model.SiteGreenJobsBoard)
	}
	if j1.DatePosted == nil {
		t.Error("job[0].DatePosted is nil")
	}

	// Second job should fallback to description since no content:encoded
	j2 := jobs[1]
	if j2.ID != "greenjobsboard-env-officer" {
		t.Errorf("job[1].ID = %q, want %q", j2.ID, "greenjobsboard-env-officer")
	}
	if j2.CompanyName != "EcoCorp" {
		t.Errorf("job[1].CompanyName = %q, want %q", j2.CompanyName, "EcoCorp")
	}
	if j2.Description != "Short description" {
		t.Errorf("job[1].Description = %q, want %q", j2.Description, "Short description")
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
		SearchTerm:    "eco",
		ResultsWanted: 25,
	})
	if err != nil {
		t.Fatalf("Scrape() returned error: %v", err)
	}

	if len(jobs) != 1 {
		t.Fatalf("expected 1 job matching 'eco', got %d", len(jobs))
	}
	if jobs[0].CompanyName != "EcoCorp" {
		t.Errorf("job[0].CompanyName = %q, want %q", jobs[0].CompanyName, "EcoCorp")
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
