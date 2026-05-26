package guardianjobs

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
  <title>Guardian Jobs</title>
  <item>
    <title>GreenEarth Inc.: Sustainability Manager</title>
    <link>https://jobs.theguardian.com/job/10037831/programme-manager-nature-awards/</link>
    <guid>https://jobs.theguardian.com/job/10037831/programme-manager-nature-awards/</guid>
    <description>Lead sustainability initiatives for a growing company.</description>
    <pubDate>Mon, 15 May 2026 10:00:00 GMT</pubDate>
  </item>
  <item>
    <title>TechOrg: Senior Developer</title>
    <link>https://jobs.theguardian.com/job/10037832/senior-developer/</link>
    <guid>https://jobs.theguardian.com/job/10037832/senior-developer/</guid>
    <description>Build software solutions</description>
    <pubDate>Tue, 16 May 2026 14:30:00 GMT</pubDate>
  </item>
  <item>
    <title></title>
    <link>https://jobs.theguardian.com/job/empty/</link>
    <guid>https://jobs.theguardian.com/job/empty/</guid>
    <description>Empty</description>
    <pubDate>Wed, 17 May 2026 08:00:00 GMT</pubDate>
  </item>
</channel>
</rss>`

func TestScraper_SiteName(t *testing.T) {
	s := New(nil)
	if got := s.SiteName(); got != model.SiteGuardianJobs {
		t.Errorf("SiteName() = %q, want %q", got, model.SiteGuardianJobs)
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

	// First job with company parsing from title "COMPANY: Job Title"
	j1 := jobs[0]
	if j1.ID != "guardianjobs-10037831" {
		t.Errorf("job[0].ID = %q, want %q", j1.ID, "guardianjobs-10037831")
	}
	if j1.Title != "Sustainability Manager" {
		t.Errorf("job[0].Title = %q, want %q", j1.Title, "Sustainability Manager")
	}
	if j1.CompanyName != "GreenEarth Inc." {
		t.Errorf("job[0].CompanyName = %q, want %q", j1.CompanyName, "GreenEarth Inc.")
	}
	if j1.JobURL != "https://jobs.theguardian.com/job/10037831/programme-manager-nature-awards/" {
		t.Errorf("job[0].JobURL = %q", j1.JobURL)
	}
	if j1.Site != string(model.SiteGuardianJobs) {
		t.Errorf("job[0].Site = %q, want %q", j1.Site, model.SiteGuardianJobs)
	}
	if j1.DatePosted == nil {
		t.Error("job[0].DatePosted is nil")
	}

	// Second job
	j2 := jobs[1]
	if j2.ID != "guardianjobs-10037832" {
		t.Errorf("job[1].ID = %q, want %q", j2.ID, "guardianjobs-10037832")
	}
	if j2.Title != "Senior Developer" {
		t.Errorf("job[1].Title = %q, want %q", j2.Title, "Senior Developer")
	}
	if j2.CompanyName != "TechOrg" {
		t.Errorf("job[1].CompanyName = %q, want %q", j2.CompanyName, "TechOrg")
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
		SearchTerm:    "developer",
		ResultsWanted: 25,
	})
	if err != nil {
		t.Fatalf("Scrape() returned error: %v", err)
	}

	if len(jobs) != 1 {
		t.Fatalf("expected 1 job matching 'developer', got %d", len(jobs))
	}
	if jobs[0].CompanyName != "TechOrg" {
		t.Errorf("job[0].CompanyName = %q, want %q", jobs[0].CompanyName, "TechOrg")
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
