package pythonjobs

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
  <title>Python Jobs</title>
  <item>
    <title>Senior Python Engineer</title>
    <link>https://www.python.org/jobs/senior-python-engineer/</link>
    <guid>https://www.python.org/jobs/senior-python-engineer/</guid>
    <description>&lt;p&gt;Build Python backends.&lt;/p&gt;</description>
    <category>Backend</category>
    <pubDate>Mon, 15 May 2026 10:00:00 GMT</pubDate>
  </item>
  <item>
    <title>ML Engineer</title>
    <link>https://www.python.org/jobs/ml-engineer/</link>
    <guid>https://www.python.org/jobs/ml-engineer/</guid>
    <description><![CDATA[<p>Machine learning with Python.</p>]]></description>
    <category>Data Science</category>
    <pubDate>Tue, 16 May 2026 14:30:00 GMT</pubDate>
  </item>
  <item>
    <title></title>
    <link>https://www.python.org/jobs/empty-title/</link>
    <guid>https://www.python.org/jobs/empty-title/</guid>
    <description>Empty title job</description>
    <category></category>
    <pubDate>Wed, 17 May 2026 08:00:00 GMT</pubDate>
  </item>
</channel>
</rss>`

func TestScraper_SiteName(t *testing.T) {
	s := New(nil)
	if got := s.SiteName(); got != model.SitePythonJobs {
		t.Errorf("SiteName() = %q, want %q", got, model.SitePythonJobs)
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
	if j1.ID != "pythonjobs-senior-python-engineer" {
		t.Errorf("job[0].ID = %q, want %q", j1.ID, "pythonjobs-senior-python-engineer")
	}
	if j1.Title != "Senior Python Engineer" {
		t.Errorf("job[0].Title = %q, want %q", j1.Title, "Senior Python Engineer")
	}
	if j1.JobURL != "https://www.python.org/jobs/senior-python-engineer/" {
		t.Errorf("job[0].JobURL = %q", j1.JobURL)
	}
	if j1.Site != string(model.SitePythonJobs) {
		t.Errorf("job[0].Site = %q, want %q", j1.Site, model.SitePythonJobs)
	}

	j2 := jobs[1]
	if j2.ID != "pythonjobs-ml-engineer" {
		t.Errorf("job[1].ID = %q, want %q", j2.ID, "pythonjobs-ml-engineer")
	}
	if j2.Title != "ML Engineer" {
		t.Errorf("job[1].Title = %q, want %q", j2.Title, "ML Engineer")
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
		SearchTerm:    "ml",
		ResultsWanted: 25,
	})
	if err != nil {
		t.Fatalf("Scrape() returned error: %v", err)
	}

	if len(jobs) != 1 {
		t.Fatalf("expected 1 job matching 'ml', got %d", len(jobs))
	}
	if jobs[0].Title != "ML Engineer" {
		t.Errorf("job[0].Title = %q, want %q", jobs[0].Title, "ML Engineer")
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
