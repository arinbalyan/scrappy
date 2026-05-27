package conservationjobs

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
  <title>Conservation Job Board</title>
  <item>
    <title>Park Ranger</title>
    <link>https://www.conservationjobboard.com/job/park-ranger-123/</link>
    <guid>https://www.conservationjobboard.com/job/park-ranger-123/</guid>
    <description>&lt;p&gt;Protect and preserve national park resources.&lt;/p&gt;</description>
    <pubDate>Mon, 15 May 2026 10:00:00 GMT</pubDate>
  </item>
  <item>
    <title>Wildlife Biologist</title>
    <link>https://www.conservationjobboard.com/job/wildlife-biologist-456/</link>
    <guid>https://www.conservationjobboard.com/job/wildlife-biologist-456/</guid>
    <description><![CDATA[<p>Study wildlife populations and habitats.</p>]]></description>
    <pubDate>Tue, 16 May 2026 14:30:00 GMT</pubDate>
  </item>
  <item>
    <title></title>
    <link>https://www.conservationjobboard.com/job/empty-title/</link>
    <guid>https://www.conservationjobboard.com/job/empty-title/</guid>
    <description>Empty title job</description>
    <pubDate>Wed, 17 May 2026 08:00:00 GMT</pubDate>
  </item>
</channel>
</rss>`

func TestScraper_SiteName(t *testing.T) {
	s := New(nil)
	if got := s.SiteName(); got != model.SiteConservationJobs {
		t.Errorf("SiteName() = %q, want %q", got, model.SiteConservationJobs)
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
	if j0.ID != "conservationjobs-park-ranger-123" {
		t.Errorf("job[0].ID = %q, want %q", j0.ID, "conservationjobs-park-ranger-123")
	}
	if j0.Title != "Park Ranger" {
		t.Errorf("job[0].Title = %q, want %q", j0.Title, "Park Ranger")
	}
	if j0.JobURL != "https://www.conservationjobboard.com/job/park-ranger-123/" {
		t.Errorf("job[0].JobURL = %q", j0.JobURL)
	}
	if j0.Site != string(model.SiteConservationJobs) {
		t.Errorf("job[0].Site = %q, want %q", j0.Site, model.SiteConservationJobs)
	}
	if j0.DatePosted == nil {
		t.Error("job[0].DatePosted is nil")
	} else if !j0.DatePosted.Before(time.Now()) {
		t.Error("job[0].DatePosted should be in the past")
	}

	// Check second job
	j1 := jobs[1]
	if j1.ID != "conservationjobs-wildlife-biologist-456" {
		t.Errorf("job[1].ID = %q", j1.ID)
	}
	if j1.Title != "Wildlife Biologist" {
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
		SearchTerm:    "wildlife",
		ResultsWanted: 25,
	})
	if err != nil {
		t.Fatalf("Scrape() returned error: %v", err)
	}

	if len(jobs) != 1 {
		t.Fatalf("expected 1 job matching 'wildlife', got %d", len(jobs))
	}
	if jobs[0].Title != "Wildlife Biologist" {
		t.Errorf("job[0].Title = %q, want %q", jobs[0].Title, "Wildlife Biologist")
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
