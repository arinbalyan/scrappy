package jobsinjapan

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
  <title>Jobs in Japan</title>
  <item>
    <title>English Teacher</title>
    <link>https://jobsinjapan.com/jobs/english-teacher-123</link>
    <guid>https://jobsinjapan.com/jobs/english-teacher-123</guid>
    <description>&lt;p&gt;Teach English at a private school in Tokyo.&lt;/p&gt;</description>
    <pubDate>Mon, 15 May 2026 10:00:00 GMT</pubDate>
    <company>Eikaiwa School</company>
    <job_type>Full-time</job_type>
    <job_address>Tokyo, Japan</job_address>
    <_salary>¥3,000,000 - ¥4,000,000</_salary>
  </item>
  <item>
    <title>Software Engineer</title>
    <link>https://jobsinjapan.com/jobs/software-engineer-456</link>
    <guid>https://jobsinjapan.com/jobs/software-engineer-456</guid>
    <description><![CDATA[<p>Full-stack development at a startup in Osaka.</p>]]></description>
    <pubDate>Tue, 16 May 2026 14:30:00 GMT</pubDate>
    <company>TechStartup KK</company>
    <job_type>Full-time</job_type>
    <job_address>Osaka, Japan</job_address>
    <_salary>¥6,000,000 - ¥8,000,000</_salary>
  </item>
  <item>
    <title></title>
    <link>https://jobsinjapan.com/jobs/empty-job</link>
    <guid>https://jobsinjapan.com/jobs/empty-job</guid>
    <description>Empty title</description>
    <pubDate>Wed, 17 May 2026 08:00:00 GMT</pubDate>
  </item>
</channel>
</rss>`

func TestScraper_SiteName(t *testing.T) {
	s := New(nil)
	if got := s.SiteName(); got != model.SiteJobsInJapan {
		t.Errorf("SiteName() = %q, want %q", got, model.SiteJobsInJapan)
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
	if j0.ID != "jobsinjapan-english-teacher-123" {
		t.Errorf("job[0].ID = %q, want %q", j0.ID, "jobsinjapan-english-teacher-123")
	}
	if j0.Title != "English Teacher" {
		t.Errorf("job[0].Title = %q", j0.Title)
	}
	if j0.CompanyName != "Eikaiwa School" {
		t.Errorf("job[0].CompanyName = %q", j0.CompanyName)
	}
	if j0.JobURL != "https://jobsinjapan.com/jobs/english-teacher-123" {
		t.Errorf("job[0].JobURL = %q", j0.JobURL)
	}
	if j0.Location.City != "Tokyo, Japan" {
		t.Errorf("job[0].Location.City = %q", j0.Location.City)
	}
	if j0.Location.Country != "Japan" {
		t.Errorf("job[0].Location.Country = %q", j0.Location.Country)
	}
	if j0.Site != string(model.SiteJobsInJapan) {
		t.Errorf("job[0].Site = %q", j0.Site)
	}
	if j0.DatePosted == nil {
		t.Error("job[0].DatePosted is nil")
	} else if !j0.DatePosted.Before(time.Now()) {
		t.Error("job[0].DatePosted should be in the past")
	}

	// Check second job
	j1 := jobs[1]
	if j1.ID != "jobsinjapan-software-engineer-456" {
		t.Errorf("job[1].ID = %q", j1.ID)
	}
	if j1.Title != "Software Engineer" {
		t.Errorf("job[1].Title = %q", j1.Title)
	}
	if j1.CompanyName != "TechStartup KK" {
		t.Errorf("job[1].CompanyName = %q", j1.CompanyName)
	}
	if j1.Location.City != "Osaka, Japan" {
		t.Errorf("job[1].Location.City = %q", j1.Location.City)
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
		SearchTerm:    "Software Engineer",
		ResultsWanted: 25,
	})
	if err != nil {
		t.Fatalf("Scrape() returned error: %v", err)
	}

	if len(jobs) != 1 {
		t.Fatalf("expected 1 job matching 'Software Engineer', got %d", len(jobs))
	}
	if jobs[0].Title != "Software Engineer" {
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
