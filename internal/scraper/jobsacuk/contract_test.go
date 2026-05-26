package jobsacuk

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
  <title>Jobs.ac.uk</title>
  <item>
    <title>Senior Research Fellow</title>
    <link>https://www.jobs.ac.uk/job/senior-research-fellow-123</link>
    <guid>https://www.jobs.ac.uk/job/senior-research-fellow-123</guid>
    <description>&lt;p&gt;Research position in computational biology.&lt;/p&gt;</description>
    <pubDate>Mon, 15 May 2026 10:00:00 GMT</pubDate>
  </item>
  <item>
    <title>Lecturer in Computer Science</title>
    <link>https://www.jobs.ac.uk/job/lecturer-cs-456</link>
    <guid>https://www.jobs.ac.uk/job/lecturer-cs-456</guid>
    <description><![CDATA[<p>Teaching and research in AI and machine learning.</p>]]></description>
    <pubDate>Tue, 16 May 2026 14:30:00 GMT</pubDate>
  </item>
  <item>
    <title></title>
    <link>https://www.jobs.ac.uk/job/empty-title</link>
    <guid>https://www.jobs.ac.uk/job/empty-title</guid>
    <description>Empty title job</description>
    <pubDate>Wed, 17 May 2026 08:00:00 GMT</pubDate>
  </item>
</channel>
</rss>`

func TestScraper_SiteName(t *testing.T) {
	s := New(nil)
	if got := s.SiteName(); got != model.SiteJobsAcUK {
		t.Errorf("SiteName() = %q, want %q", got, model.SiteJobsAcUK)
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
	if j0.ID != "jobsacuk-senior-research-fellow-123" {
		t.Errorf("job[0].ID = %q, want %q", j0.ID, "jobsacuk-senior-research-fellow-123")
	}
	if j0.Title != "Senior Research Fellow" {
		t.Errorf("job[0].Title = %q", j0.Title)
	}
	if j0.JobURL != "https://www.jobs.ac.uk/job/senior-research-fellow-123" {
		t.Errorf("job[0].JobURL = %q", j0.JobURL)
	}
	if j0.Site != string(model.SiteJobsAcUK) {
		t.Errorf("job[0].Site = %q", j0.Site)
	}
	if j0.Location.Country != "United Kingdom" {
		t.Errorf("job[0].Location.Country = %q", j0.Location.Country)
	}
	if j0.DatePosted == nil {
		t.Error("job[0].DatePosted is nil")
	} else if !j0.DatePosted.Before(time.Now()) {
		t.Error("job[0].DatePosted should be in the past")
	}

	// Check second job
	j1 := jobs[1]
	if j1.ID != "jobsacuk-lecturer-cs-456" {
		t.Errorf("job[1].ID = %q, want %q", j1.ID, "jobsacuk-lecturer-cs-456")
	}
	if j1.Title != "Lecturer in Computer Science" {
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
		SearchTerm:    "Senior Research Fellow",
		ResultsWanted: 25,
	})
	if err != nil {
		t.Fatalf("Scrape() returned error: %v", err)
	}

	if len(jobs) != 1 {
		t.Fatalf("expected 1 job matching 'research', got %d", len(jobs))
	}
	if jobs[0].Title != "Senior Research Fellow" {
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
