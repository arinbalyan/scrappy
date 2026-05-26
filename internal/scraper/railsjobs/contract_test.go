package railsjobs

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
  <title>Rails Jobs</title>
  <item>
    <title>Ruby on Rails Developer</title>
    <link>https://jobs.rubyonrails.org/jobs/ruby-on-rails-developer/</link>
    <guid>https://jobs.rubyonrails.org/jobs/ruby-on-rails-developer/</guid>
    <description>&lt;p&gt;Build Rails applications.&lt;/p&gt;</description>
    <pubDate>Mon, 15 May 2026 10:00:00 GMT</pubDate>
  </item>
  <item>
    <title>Senior Rails Engineer</title>
    <link>https://jobs.rubyonrails.org/jobs/senior-rails-engineer/</link>
    <guid>https://jobs.rubyonrails.org/jobs/senior-rails-engineer/</guid>
    <description><![CDATA[<p>Lead Rails development team.</p>]]></description>
    <pubDate>Tue, 16 May 2026 14:30:00 GMT</pubDate>
  </item>
  <item>
    <title></title>
    <link>https://jobs.rubyonrails.org/jobs/empty-title/</link>
    <guid>https://jobs.rubyonrails.org/jobs/empty-title/</guid>
    <description>Empty title job</description>
    <pubDate>Wed, 17 May 2026 08:00:00 GMT</pubDate>
  </item>
</channel>
</rss>`

func TestScraper_SiteName(t *testing.T) {
	s := New(nil)
	if got := s.SiteName(); got != model.SiteRailsJobs {
		t.Errorf("SiteName() = %q, want %q", got, model.SiteRailsJobs)
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
	if j1.ID != "railsjobs-ruby-on-rails-developer" {
		t.Errorf("job[0].ID = %q, want %q", j1.ID, "railsjobs-ruby-on-rails-developer")
	}
	if j1.Title != "Ruby on Rails Developer" {
		t.Errorf("job[0].Title = %q, want %q", j1.Title, "Ruby on Rails Developer")
	}
	if j1.JobURL != "https://jobs.rubyonrails.org/jobs/ruby-on-rails-developer/" {
		t.Errorf("job[0].JobURL = %q", j1.JobURL)
	}
	if j1.Site != string(model.SiteRailsJobs) {
		t.Errorf("job[0].Site = %q, want %q", j1.Site, model.SiteRailsJobs)
	}

	j2 := jobs[1]
	if j2.ID != "railsjobs-senior-rails-engineer" {
		t.Errorf("job[1].ID = %q, want %q", j2.ID, "railsjobs-senior-rails-engineer")
	}
	if j2.Title != "Senior Rails Engineer" {
		t.Errorf("job[1].Title = %q, want %q", j2.Title, "Senior Rails Engineer")
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
		SearchTerm:    "senior",
		ResultsWanted: 25,
	})
	if err != nil {
		t.Fatalf("Scrape() returned error: %v", err)
	}

	if len(jobs) != 1 {
		t.Fatalf("expected 1 job matching 'senior', got %d", len(jobs))
	}
	if jobs[0].Title != "Senior Rails Engineer" {
		t.Errorf("job[0].Title = %q, want %q", jobs[0].Title, "Senior Rails Engineer")
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
