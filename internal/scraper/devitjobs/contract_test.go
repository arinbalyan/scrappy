package devitjobs

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
)

const testXML = `<?xml version="1.0" encoding="UTF-8"?>
<jobs>
<job>
  <title>Senior Go Developer</title>
  <link>https://devitjobs.com/jobs/senior-go-developer-123</link>
  <description><![CDATA[<p>Build scalable backend systems in Go.</p>]]></description>
  <company>TechCorp</company>
  <location>Berlin, Germany</location>
  <salary>$120,000 - 140,000 per year</salary>
  <pubDate>Mon, 15 May 2026 10:00:00 GMT</pubDate>
  <category>Engineering</category>
  <type>Full-time</type>
</job>
<job>
  <title>Machine Learning Engineer</title>
  <link>https://devitjobs.com/jobs/ml-engineer-456</link>
  <description>&lt;p&gt;Design and implement ML models.&lt;/p&gt;</description>
  <company>AI Labs</company>
  <location>Remote</location>
  <salary>€80,000 - 100,000 per year</salary>
  <pubDate>Tue, 16 May 2026 14:30:00 GMT</pubDate>
  <category>AI</category>
  <type>Remote</type>
</job>
<job>
  <title></title>
  <link>https://devitjobs.com/jobs/empty-title</link>
  <description>Empty title job</description>
  <pubDate>Wed, 17 May 2026 08:00:00 GMT</pubDate>
</job>
</jobs>`

func TestScraper_SiteName(t *testing.T) {
	s := New(nil)
	if got := s.SiteName(); got != model.SiteDevITJobs {
		t.Errorf("SiteName() = %q, want %q", got, model.SiteDevITJobs)
	}
}

func TestScraper_Scrape(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(testXML))
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
	if j0.ID != "devitjobs-senior-go-developer-123" {
		t.Errorf("job[0].ID = %q, want %q", j0.ID, "devitjobs-senior-go-developer-123")
	}
	if j0.Title != "Senior Go Developer" {
		t.Errorf("job[0].Title = %q, want %q", j0.Title, "Senior Go Developer")
	}
	if j0.CompanyName != "TechCorp" {
		t.Errorf("job[0].CompanyName = %q, want %q", j0.CompanyName, "TechCorp")
	}
	if j0.Location.City != "Berlin, Germany" {
		t.Errorf("job[0].Location.City = %q, want %q", j0.Location.City, "Berlin, Germany")
	}
	if j0.Site != string(model.SiteDevITJobs) {
		t.Errorf("job[0].Site = %q", j0.Site)
	}
	if j0.Compensation == nil {
		t.Fatal("job[0].Compensation is nil")
	}
	if j0.Compensation.Currency != "USD" {
		t.Errorf("job[0].Compensation.Currency = %q, want %q", j0.Compensation.Currency, "USD")
	}
	if j0.Compensation.MinAmount == nil || *j0.Compensation.MinAmount != 120000 {
		t.Errorf("job[0].Compensation.MinAmount = %v, want 120000", j0.Compensation.MinAmount)
	}
	if j0.IsRemote {
		t.Errorf("job[0].IsRemote should be false")
	}
	if j0.DatePosted == nil {
		t.Error("job[0].DatePosted is nil")
	} else if !j0.DatePosted.Before(time.Now()) {
		t.Error("job[0].DatePosted should be in the past")
	}

	// Check second job (remote with EUR salary)
	j1 := jobs[1]
	if j1.ID != "devitjobs-ml-engineer-456" {
		t.Errorf("job[1].ID = %q", j1.ID)
	}
	if j1.CompanyName != "AI Labs" {
		t.Errorf("job[1].CompanyName = %q", j1.CompanyName)
	}
	if j1.Compensation == nil {
		t.Fatal("job[1].Compensation is nil")
	}
	if j1.Compensation.Currency != "EUR" {
		t.Errorf("job[1].Compensation.Currency = %q, want %q", j1.Compensation.Currency, "EUR")
	}
	if !j1.IsRemote {
		t.Errorf("job[1].IsRemote should be true")
	}
}

func TestScraper_Scrape_SearchTerm(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(testXML))
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
	if jobs[0].Title != "Machine Learning Engineer" {
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
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`<?xml version="1.0"?><jobs></jobs>`))
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
