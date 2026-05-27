package berlinstartupjobs

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
  <title>Berlin Startup Jobs</title>
  <item>
    <title>Senior Go Developer // TechCorp Berlin</title>
    <link>https://berlinstartupjobs.com/engineering/senior-go-developer/</link>
    <guid>https://berlinstartupjobs.com/engineering/senior-go-developer/</guid>
    <description>&lt;p&gt;Build scalable backend systems in Go.&lt;/p&gt;</description>
    <category>Engineering</category>
    <pubDate>Mon, 15 May 2026 10:00:00 GMT</pubDate>
  </item>
  <item>
    <title>Machine Learning Engineer // AI Labs</title>
    <link>https://berlinstartupjobs.com/ai/machine-learning-engineer/</link>
    <guid>https://berlinstartupjobs.com/ai/machine-learning-engineer/</guid>
    <description><![CDATA[<p>Design and implement ML models.</p>]]></description>
    <category>AI</category>
    <pubDate>Tue, 16 May 2026 14:30:00 GMT</pubDate>
  </item>
  <item>
    <title></title>
    <link>https://berlinstartupjobs.com/engineering/empty-title/</link>
    <guid>https://berlinstartupjobs.com/engineering/empty-title/</guid>
    <description>Empty title job</description>
    <pubDate>Wed, 17 May 2026 08:00:00 GMT</pubDate>
  </item>
</channel>
</rss>`

func TestScraper_SiteName(t *testing.T) {
	s := New(nil)
	if got := s.SiteName(); got != model.SiteBerlinStartupJobs {
		t.Errorf("SiteName() = %q, want %q", got, model.SiteBerlinStartupJobs)
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
	if j0.ID != "berlinstartupjobs-senior-go-developer" {
		t.Errorf("job[0].ID = %q, want %q", j0.ID, "berlinstartupjobs-senior-go-developer")
	}
	if j0.Title != "Senior Go Developer // TechCorp Berlin" {
		t.Errorf("job[0].Title = %q", j0.Title)
	}
	if j0.CompanyName != "TechCorp Berlin" {
		t.Errorf("job[0].CompanyName = %q, want %q", j0.CompanyName, "TechCorp Berlin")
	}
	if j0.JobURL != "https://berlinstartupjobs.com/engineering/senior-go-developer/" {
		t.Errorf("job[0].JobURL = %q", j0.JobURL)
	}
	if j0.Site != string(model.SiteBerlinStartupJobs) {
		t.Errorf("job[0].Site = %q", j0.Site)
	}
	if j0.DatePosted == nil {
		t.Error("job[0].DatePosted is nil")
	} else if !j0.DatePosted.Before(time.Now()) {
		t.Error("job[0].DatePosted should be in the past")
	}

	// Check second job
	j1 := jobs[1]
	if j1.ID != "berlinstartupjobs-machine-learning-engineer" {
		t.Errorf("job[1].ID = %q", j1.ID)
	}
	if j1.CompanyName != "AI Labs" {
		t.Errorf("job[1].CompanyName = %q, want %q", j1.CompanyName, "AI Labs")
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
		SearchTerm:    "machine",
		ResultsWanted: 25,
	})
	if err != nil {
		t.Fatalf("Scrape() returned error: %v", err)
	}

	if len(jobs) != 1 {
		t.Fatalf("expected 1 job matching 'machine', got %d", len(jobs))
	}
	if jobs[0].Title != "Machine Learning Engineer // AI Labs" {
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
