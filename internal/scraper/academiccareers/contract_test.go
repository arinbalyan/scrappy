package academiccareers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/arinbalyan/scrappy/internal/model"
)

const testRSSFeed = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
<channel>
  <title>Academic Careers</title>
  <item>
    <title>Assistant Professor in Computer Science</title>
    <link>https://www.academiccareers.com/jobs/12345/assistant-professor-cs</link>
    <guid isPermaLink="false">abc-12345</guid>
    <description><![CDATA[<p>The Department of Computer Science at University of Excellence invites applications for a tenure-track Assistant Professor position.</p>]]></description>
    <pubDate>Mon, 25 May 2026 09:00:00 GMT</pubDate>
    <dc:creator>University of Excellence</dc:creator>
  </item>
  <item>
    <title>Research Scientist - Bioinformatics</title>
    <link>https://www.academiccareers.com/jobs/67890/research-scientist-bio</link>
    <guid isPermaLink="false">def-67890</guid>
    <description><![CDATA[<p>Join the Bioinformatics Lab at Research Institute. Leading-edge genomics research.</p>]]></description>
    <pubDate>Sun, 24 May 2026 14:30:00 GMT</pubDate>
    <dc:creator>Research Institute of Biology</dc:creator>
  </item>
  <item>
    <title>Dean of Engineering</title>
    <link></link>
    <guid isPermaLink="false">ghi-11111</guid>
    <description>Dean position at State University.</description>
    <pubDate></pubDate>
    <dc:creator>State University</dc:creator>
  </item>
  <item>
    <title></title>
    <link>https://www.academiccareers.com/jobs/empty/empty-title</link>
    <guid></guid>
    <description></description>
    <pubDate></pubDate>
    <dc:creator></dc:creator>
  </item>
</channel>
</rss>`

func TestScraper_SiteName(t *testing.T) {
	s := New(nil)
	if got := s.SiteName(); got != model.SiteAcademicCareers {
		t.Errorf("SiteName() = %q, want %q", got, model.SiteAcademicCareers)
	}
}

func TestScraper_Scrape(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(testRSSFeed))
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
	if !strings.HasPrefix(j1.ID, "academiccareers-") {
		t.Errorf("job[0].ID = %q, want prefix 'academiccareers-'", j1.ID)
	}
	if j1.Title != "Assistant Professor in Computer Science" {
		t.Errorf("job[0].Title = %q, want %q", j1.Title, "Assistant Professor in Computer Science")
	}
	if j1.CompanyName != "University of Excellence" {
		t.Errorf("job[0].CompanyName = %q, want %q", j1.CompanyName, "University of Excellence")
	}
	if j1.JobURL != "https://www.academiccareers.com/jobs/12345/assistant-professor-cs" {
		t.Errorf("job[0].JobURL = %q", j1.JobURL)
	}
	// Description should be stripped of HTML
	if strings.Contains(j1.Description, "<p>") {
		t.Errorf("job[0].Description still contains HTML tags: %q", j1.Description)
	}
	if !strings.Contains(j1.Description, "Assistant Professor") {
		t.Errorf("job[0].Description = %q, expected to contain 'Assistant Professor'", j1.Description)
	}
	if j1.DatePosted == nil {
		t.Error("job[0].DatePosted is nil, expected a parsed date")
	}
	if j1.Site != string(model.SiteAcademicCareers) {
		t.Errorf("job[0].Site = %q, want %q", j1.Site, model.SiteAcademicCareers)
	}

	j2 := jobs[1]
	if !strings.HasPrefix(j2.ID, "academiccareers-") {
		t.Errorf("job[1].ID = %q, want prefix 'academiccareers-'", j1.ID)
	}
	if j2.Title != "Research Scientist - Bioinformatics" {
		t.Errorf("job[1].Title = %q, want %q", j2.Title, "Research Scientist - Bioinformatics")
	}
	if j2.CompanyName != "Research Institute of Biology" {
		t.Errorf("job[1].CompanyName = %q, want %q", j2.CompanyName, "Research Institute of Biology")
	}
	if j2.DatePosted == nil {
		t.Error("job[1].DatePosted is nil, expected a parsed date")
	}
}

func TestScraper_Scrape_WithSearchTerm(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(testRSSFeed))
	}))
	defer ts.Close()

	s := NewWithFeedURL(nil, ts.URL)
	jobs, err := s.Scrape(context.Background(), model.ScraperInput{
		SearchTerm:    "Research",
		ResultsWanted: 25,
	})
	if err != nil {
		t.Fatalf("Scrape() returned error: %v", err)
	}

	if len(jobs) != 1 {
		t.Fatalf("expected 1 job matching 'Research', got %d", len(jobs))
	}
	if j2 := jobs[0]; j2.Title != "Research Scientist - Bioinformatics" {
		t.Errorf("expected 'Research Scientist - Bioinformatics', got %q", j2.Title)
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

func TestScraper_Scrape_EmptyFeed(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`<?xml version="1.0"?><rss version="2.0"><channel><title>Empty</title></channel></rss>`))
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
