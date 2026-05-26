package swissdevjobs

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
  <title>SwissDevJobs</title>
  <item>
    <title>Senior Go Developer @ TechCorp GmbH [CHF 120'000 - 160'000]</title>
    <link>https://swissdevjobs.ch/jobs/senior-go-developer-techcorp</link>
    <guid>https://swissdevjobs.ch/jobs/senior-go-developer-techcorp</guid>
    <description>&lt;p&gt;Build Go applications in Zurich.&lt;/p&gt;</description>
    <category>Backend</category>
    <pubDate>Mon, 15 May 2026 10:00:00 GMT</pubDate>
  </item>
  <item>
    <title>Frontend Developer @ WebStudio AG [CHF 90'000 - 130'000]</title>
    <link>https://swissdevjobs.ch/jobs/frontend-developer-webstudio</link>
    <guid>https://swissdevjobs.ch/jobs/frontend-developer-webstudio</guid>
    <description><![CDATA[<p>Build React UIs in Bern.</p>]]></description>
    <category>Frontend</category>
    <pubDate>Tue, 16 May 2026 14:30:00 GMT</pubDate>
  </item>
  <item>
    <title></title>
    <link>https://swissdevjobs.ch/jobs/empty-title</link>
    <guid>https://swissdevjobs.ch/jobs/empty-title</guid>
    <description>Empty title job</description>
    <category></category>
    <pubDate>Wed, 17 May 2026 08:00:00 GMT</pubDate>
  </item>
</channel>
</rss>`

func TestScraper_SiteName(t *testing.T) {
	s := New(nil)
	if got := s.SiteName(); got != model.SiteSwissDevJobs {
		t.Errorf("SiteName() = %q, want %q", got, model.SiteSwissDevJobs)
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
	j1 := jobs[0]
	if j1.ID != "swissdevjobs-senior-go-developer-techcorp" {
		t.Errorf("job[0].ID = %q, want %q", j1.ID, "swissdevjobs-senior-go-developer-techcorp")
	}
	if j1.Title != "Senior Go Developer @ TechCorp GmbH [CHF 120'000 - 160'000]" {
		t.Errorf("job[0].Title = %q", j1.Title)
	}
	if j1.CompanyName != "TechCorp GmbH" {
		t.Errorf("job[0].CompanyName = %q, want %q", j1.CompanyName, "TechCorp GmbH")
	}
	if j1.JobURL != "https://swissdevjobs.ch/jobs/senior-go-developer-techcorp" {
		t.Errorf("job[0].JobURL = %q", j1.JobURL)
	}
	if j1.Site != string(model.SiteSwissDevJobs) {
		t.Errorf("job[0].Site = %q, want %q", j1.Site, model.SiteSwissDevJobs)
	}
	if j1.Compensation == nil {
		t.Fatal("job[0].Compensation is nil, expected salary from title")
	}
	if j1.Compensation.Currency != "CHF" {
		t.Errorf("job[0].Compensation.Currency = %q, want %q", j1.Compensation.Currency, "CHF")
	}
	if j1.Compensation.MinAmount == nil || *j1.Compensation.MinAmount != 120000 {
		t.Errorf("job[0].Compensation.MinAmount = %v, want 120000", j1.Compensation.MinAmount)
	}
	if j1.Compensation.MaxAmount == nil || *j1.Compensation.MaxAmount != 160000 {
		t.Errorf("job[0].Compensation.MaxAmount = %v, want 160000", j1.Compensation.MaxAmount)
	}
	if j1.Location.Country != "Switzerland" {
		t.Errorf("job[0].Location.Country = %q, want %q", j1.Location.Country, "Switzerland")
	}
	if j1.DatePosted == nil {
		t.Error("job[0].DatePosted is nil, expected a parsed date")
	}

	// Check second job
	j2 := jobs[1]
	if j2.ID != "swissdevjobs-frontend-developer-webstudio" {
		t.Errorf("job[1].ID = %q, want %q", j2.ID, "swissdevjobs-frontend-developer-webstudio")
	}
	if j2.Title != "Frontend Developer @ WebStudio AG [CHF 90'000 - 130'000]" {
		t.Errorf("job[1].Title = %q", j2.Title)
	}
	if j2.CompanyName != "WebStudio AG" {
		t.Errorf("job[1].CompanyName = %q, want %q", j2.CompanyName, "WebStudio AG")
	}
	if j2.Compensation.MinAmount == nil || *j2.Compensation.MinAmount != 90000 {
		t.Errorf("job[1].Compensation.MinAmount = %v, want 90000", j2.Compensation.MinAmount)
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
		SearchTerm:    "frontend",
		ResultsWanted: 25,
	})
	if err != nil {
		t.Fatalf("Scrape() returned error: %v", err)
	}

	if len(jobs) != 1 {
		t.Fatalf("expected 1 job matching 'frontend', got %d", len(jobs))
	}
	if jobs[0].CompanyName != "WebStudio AG" {
		t.Errorf("job[0].CompanyName = %q, want %q", jobs[0].CompanyName, "WebStudio AG")
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
