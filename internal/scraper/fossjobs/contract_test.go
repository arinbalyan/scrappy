package fossjobs

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
  <title>FOSS Jobs</title>
  <item>
    <title>Linux Kernel Developer</title>
    <link>https://www.fossjobs.net/job/linux-kernel-dev/</link>
    <guid>https://www.fossjobs.net/job/linux-kernel-dev/</guid>
    <description>&lt;p&gt;Develop and maintain Linux kernel modules.&lt;/p&gt;</description>
    <pubDate>Mon, 15 May 2026 10:00:00 GMT</pubDate>
    <category>kernel</category>
  </item>
  <item>
    <title>Python Backend Engineer</title>
    <link>https://www.fossjobs.net/job/python-backend/</link>
    <guid>https://www.fossjobs.net/job/python-backend/</guid>
    <description><![CDATA[<p>Build open source Python tools.</p>]]></description>
    <pubDate>Tue, 16 May 2026 14:30:00 GMT</pubDate>
    <category>python</category>
  </item>
  <item>
    <title></title>
    <link>https://www.fossjobs.net/job/empty-title/</link>
    <guid>https://www.fossjobs.net/job/empty-title/</guid>
    <description>Empty title job</description>
    <pubDate>Wed, 17 May 2026 08:00:00 GMT</pubDate>
    <category>misc</category>
  </item>
</channel>
</rss>`

func TestScraper_SiteName(t *testing.T) {
	s := New(nil)
	if got := s.SiteName(); got != model.SiteFossJobs {
		t.Errorf("SiteName() = %q, want %q", got, model.SiteFossJobs)
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
	if j1.ID != "fossjobs-linux-kernel-dev" {
		t.Errorf("job[0].ID = %q, want %q", j1.ID, "fossjobs-linux-kernel-dev")
	}
	if j1.Title != "Linux Kernel Developer" {
		t.Errorf("job[0].Title = %q, want %q", j1.Title, "Linux Kernel Developer")
	}
	if j1.JobURL != "https://www.fossjobs.net/job/linux-kernel-dev/" {
		t.Errorf("job[0].JobURL = %q", j1.JobURL)
	}
	if j1.Site != string(model.SiteFossJobs) {
		t.Errorf("job[0].Site = %q, want %q", j1.Site, model.SiteFossJobs)
	}
	if j1.DatePosted == nil {
		t.Error("job[0].DatePosted is nil, expected a parsed date")
	}

	j2 := jobs[1]
	if j2.ID != "fossjobs-python-backend" {
		t.Errorf("job[1].ID = %q, want %q", j2.ID, "fossjobs-python-backend")
	}
	if j2.Title != "Python Backend Engineer" {
		t.Errorf("job[1].Title = %q, want %q", j2.Title, "Python Backend Engineer")
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
		SearchTerm:    "python",
		ResultsWanted: 25,
	})
	if err != nil {
		t.Fatalf("Scrape() returned error: %v", err)
	}

	if len(jobs) != 1 {
		t.Fatalf("expected 1 job matching 'python', got %d", len(jobs))
	}
	if jobs[0].Title != "Python Backend Engineer" {
		t.Errorf("job[0].Title = %q, want %q", jobs[0].Title, "Python Backend Engineer")
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
