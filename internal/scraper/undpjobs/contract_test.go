package undpjobs

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
  <title>UNDP Jobs</title>
  <item>
    <title>Project Manager</title>
    <link>https://jobs.undp.org/cj_view_job.cfm?cur_job_id=12345</link>
    <description>Manage development projects in Africa.</description>
    <undpjobs:duty_station>New York, USA</undpjobs:duty_station>
    <undpjobs:closing_date>2026-06-15</undpjobs:closing_date>
    <undpjobs:organization>UNDP</undpjobs:organization>
    <dc:date>2026-05-20T10:00:00Z</dc:date>
  </item>
  <item>
    <title>Technical Advisor</title>
    <link>https://jobs.undp.org/cj_view_job.cfm?cur_job_id=67890</link>
    <description>Provide technical guidance on climate projects.</description>
    <undpjobs:duty_station>Geneva, Switzerland</undpjobs:duty_station>
    <undpjobs:organization>UNEP</undpjobs:organization>
    <dc:date>2026-05-21T14:30:00Z</dc:date>
  </item>
  <item>
    <title></title>
    <link>https://jobs.undp.org/cj_view_job.cfm?cur_job_id=99999</link>
    <description>Empty title item</description>
  </item>
</channel>
</rss>`

func TestScraper_SiteName(t *testing.T) {
	s := New(nil)
	if got := s.SiteName(); got != model.SiteUNDPJobs {
		t.Errorf("SiteName() = %q, want %q", got, model.SiteUNDPJobs)
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
	if j0.ID != "undpjobs-12345" {
		t.Errorf("job[0].ID = %q, want %q", j0.ID, "undpjobs-12345")
	}
	if j0.Title != "Project Manager" {
		t.Errorf("job[0].Title = %q, want %q", j0.Title, "Project Manager")
	}
	if j0.CompanyName != "UNDP" {
		t.Errorf("job[0].CompanyName = %q, want %q", j0.CompanyName, "UNDP")
	}
	if j0.Site != string(model.SiteUNDPJobs) {
		t.Errorf("job[0].Site = %q, want %q", j0.Site, model.SiteUNDPJobs)
	}
	if j0.Location.City != "New York, USA" {
		t.Errorf("job[0].Location.City = %q, want %q", j0.Location.City, "New York, USA")
	}
	if j0.DatePosted == nil {
		t.Error("job[0].DatePosted is nil")
	}

	// Job 1
	j1 := jobs[1]
	if j1.ID != "undpjobs-67890" {
		t.Errorf("job[1].ID = %q, want %q", j1.ID, "undpjobs-67890")
	}
	if j1.Title != "Technical Advisor" {
		t.Errorf("job[1].Title = %q, want %q", j1.Title, "Technical Advisor")
	}
	if j1.CompanyName != "UNEP" {
		t.Errorf("job[1].CompanyName = %q, want %q", j1.CompanyName, "UNEP")
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
		SearchTerm:    "climate",
		ResultsWanted: 25,
	})
	if err != nil {
		t.Fatalf("Scrape() returned error: %v", err)
	}

	if len(jobs) != 1 {
		t.Fatalf("expected 1 job matching 'climate', got %d", len(jobs))
	}
	if jobs[0].Title != "Technical Advisor" {
		t.Errorf("job[0].Title = %q, want %q", jobs[0].Title, "Technical Advisor")
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
