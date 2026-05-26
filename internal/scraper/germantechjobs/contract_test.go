package germantechjobs

import (
	"context"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/arinbalyan/scrappy/internal/model"
)

const testRSS = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
<channel>
  <title>German Tech Jobs</title>
  <item>
    <title>Senior Go Developer @ TechCorp [60.000 - 85.000 EUR]</title>
    <link>https://germantechjobs.de/job/senior-go-developer/</link>
    <guid>https://germantechjobs.de/job/senior-go-developer/</guid>
    <description>&lt;p&gt;Build cloud-native Go services.&lt;/p&gt;</description>
    <pubDate>Mon, 15 May 2026 10:00:00 GMT</pubDate>
    <category>golang</category>
  </item>
  <item>
    <title>Python Developer @ DataCo [45.000 - 65.000 EUR]</title>
    <link>https://germantechjobs.de/job/python-dev/</link>
    <guid>https://germantechjobs.de/job/python-dev/</guid>
    <description><![CDATA[<p>Work on data pipelines.</p>]]></description>
    <pubDate>Tue, 16 May 2026 14:30:00 GMT</pubDate>
    <category>python</category>
  </item>
  <item>
    <title></title>
    <link>https://germantechjobs.de/job/empty-title/</link>
    <guid>https://germantechjobs.de/job/empty-title/</guid>
    <description>Empty title job</description>
    <pubDate>Wed, 17 May 2026 08:00:00 GMT</pubDate>
    <category>misc</category>
  </item>
</channel>
</rss>`

func TestScraper_SiteName(t *testing.T) {
	s := New(nil)
	if got := s.SiteName(); got != model.SiteGermanTechJobs {
		t.Errorf("SiteName() = %q, want %q", got, model.SiteGermanTechJobs)
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

	// Check first job with salary and company parsing
	j1 := jobs[0]
	if j1.ID != "germantechjobs-senior-go-developer" {
		t.Errorf("job[0].ID = %q, want %q", j1.ID, "germantechjobs-senior-go-developer")
	}
	if j1.Title != "Senior Go Developer @ TechCorp [60.000 - 85.000 EUR]" {
		t.Errorf("job[0].Title = %q", j1.Title)
	}
	if j1.CompanyName != "TechCorp" {
		t.Errorf("job[0].CompanyName = %q, want %q", j1.CompanyName, "TechCorp")
	}
	if j1.Site != string(model.SiteGermanTechJobs) {
		t.Errorf("job[0].Site = %q, want %q", j1.Site, model.SiteGermanTechJobs)
	}
	if j1.Compensation == nil {
		t.Fatal("job[0].Compensation is nil, expected parsed salary")
	}
	if j1.Compensation.Currency != "EUR" {
		t.Errorf("job[0].Compensation.Currency = %q, want %q", j1.Compensation.Currency, "EUR")
	}
	if j1.Compensation.MinAmount == nil || math.Abs(*j1.Compensation.MinAmount-60000) > 0.01 {
		t.Errorf("job[0].Compensation.MinAmount = %v, want 60000", j1.Compensation.MinAmount)
	}
	if j1.Compensation.MaxAmount == nil || math.Abs(*j1.Compensation.MaxAmount-85000) > 0.01 {
		t.Errorf("job[0].Compensation.MaxAmount = %v, want 85000", j1.Compensation.MaxAmount)
	}
	if j1.DatePosted == nil {
		t.Error("job[0].DatePosted is nil")
	}

	// Check second job
	j2 := jobs[1]
	if j2.ID != "germantechjobs-python-dev" {
		t.Errorf("job[1].ID = %q, want %q", j2.ID, "germantechjobs-python-dev")
	}
	if j2.CompanyName != "DataCo" {
		t.Errorf("job[1].CompanyName = %q, want %q", j2.CompanyName, "DataCo")
	}
	if j2.Compensation == nil {
		t.Fatal("job[1].Compensation is nil")
	}
	if j2.Compensation.MinAmount == nil || math.Abs(*j2.Compensation.MinAmount-45000) > 0.01 {
		t.Errorf("job[1].Compensation.MinAmount = %v, want 45000", j2.Compensation.MinAmount)
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
	if jobs[0].CompanyName != "DataCo" {
		t.Errorf("job[0].CompanyName = %q, want %q", jobs[0].CompanyName, "DataCo")
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
