package undpjobs

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/arinbalyan/scrappy/internal/model"
)

const testRSS = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
<channel>
<item>
  <title>Programme Analyst</title>
  <link>https://jobs.undp.org/cj_view_job.cfm?cur_job_id=12345</link>
  <description>&lt;p&gt;Support programme implementation in development contexts.&lt;/p&gt;</description>
  <undpjobs:duty_station>New York, USA</undpjobs:duty_station>
  <undpjobs:closing_date>2026-06-15</undpjobs:closing_date>
  <undpjobs:organization>UNDP</undpjobs:organization>
  <dc:date>2026-05-20T10:00:00Z</dc:date>
</item>
<item>
  <title>Field Officer</title>
  <link>https://jobs.undp.org/cj_view_job.cfm?cur_job_id=67890</link>
  <description>Field operations management in rural areas.</description>
  <undpjobs:duty_station>Dhaka, Bangladesh</undpjobs:duty_station>
  <undpjobs:closing_date>2026-07-01</undpjobs:closing_date>
  <undpjobs:organization>UNDP Bangladesh</undpjobs:organization>
  <dc:date>2026-05-18T08:30:00Z</dc:date>
</item>
<item>
  <title></title>
  <link>https://jobs.undp.org/cj_view_job.cfm?cur_job_id=empty</link>
  <description></description>
  <undpjobs:organization></undpjobs:organization>
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
		w.Header().Set("Content-Type", "application/xml")
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

	// Job 0: Programme Analyst at UNDP
	j0 := jobs[0]
	if j0.Title != "Programme Analyst" {
		t.Errorf("job[0].Title = %q, want %q", j0.Title, "Programme Analyst")
	}
	if j0.CompanyName != "UNDP" {
		t.Errorf("job[0].CompanyName = %q, want %q", j0.CompanyName, "UNDP")
	}
	if j0.Site != string(model.SiteUNDPJobs) {
		t.Errorf("job[0].Site = %q, want %q", j0.Site, model.SiteUNDPJobs)
	}
	if !strings.Contains(j0.JobURL, "cur_job_id=12345") {
		t.Errorf("job[0].JobURL = %q, should contain cur_job_id=12345", j0.JobURL)
	}
	if j0.Location.City != "New York, USA" {
		t.Errorf("job[0].Location.City = %q, want %q", j0.Location.City, "New York, USA")
	}
	if j0.DatePosted == nil {
		t.Error("job[0].DatePosted is nil, expected a parsed date")
	}
	if j0.Description == "" {
		t.Error("job[0].Description is empty")
	}

	// Job 1: Field Officer at UNDP Bangladesh
	j1 := jobs[1]
	if j1.Title != "Field Officer" {
		t.Errorf("job[1].Title = %q, want %q", j1.Title, "Field Officer")
	}
	if j1.CompanyName != "UNDP Bangladesh" {
		t.Errorf("job[1].CompanyName = %q, want %q", j1.CompanyName, "UNDP Bangladesh")
	}
	if j1.Location.City != "Dhaka, Bangladesh" {
		t.Errorf("job[1].Location.City = %q, want %q", j1.Location.City, "Dhaka, Bangladesh")
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

func TestScraper_Scrape_EmptyResponse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`<?xml version="1.0"?><rss><channel></channel></rss>`))
	}))
	defer ts.Close()

	s := NewWithRSSURL(nil, ts.URL)
	_, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 25})
	if err == nil {
		t.Fatal("expected error for empty response, got nil")
	}
}

func TestScraper_Scrape_429(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer ts.Close()

	s := NewWithRSSURL(nil, ts.URL)
	_, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 25})
	if err == nil {
		t.Fatal("expected error for 429 status, got nil")
	}
}

func TestScraper_Scrape_SearchFilter(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(testRSS))
	}))
	defer ts.Close()

	s := NewWithRSSURL(nil, ts.URL)
	jobs, err := s.Scrape(context.Background(), model.ScraperInput{
		ResultsWanted: 25,
		SearchTerm:    "Analyst",
	})
	if err != nil {
		t.Fatalf("Scrape() returned error: %v", err)
	}

	if len(jobs) != 1 {
		t.Fatalf("expected 1 job matching 'Analyst', got %d", len(jobs))
	}
	if jobs[0].Title != "Programme Analyst" {
		t.Errorf("job.Title = %q, want %q", jobs[0].Title, "Programme Analyst")
	}
}

func TestNewWithRSSURL_EmptyEndpoint(t *testing.T) {
	s1 := NewWithRSSURL(nil, "")
	s2 := New(nil)
	if s1.rssURL != s2.rssURL {
		t.Errorf("empty endpoint should not override RSS URL")
	}
}
