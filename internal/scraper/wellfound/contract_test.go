package wellfound

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/arinbalyan/scrappy/internal/model"
)

const testHTML = `<!DOCTYPE html>
<html>
<head>
<script id="__NEXT_DATA__" type="application/json">
{"props":{"pageProps":{"jobs":[{"id":"1001","title":"Software Engineer","slug":"software-engineer-1001","company":{"name":"TechCorp","logoUrl":"https://logo.example.com/tc.png","slug":"techcorp"},"compensation":{"min":120000,"max":180000,"currency":"USD"},"locations":["San Francisco, CA"],"remote":false,"description":"<p>Build great things.</p>","skills":["Go","React"],"createdAt":"2026-05-20T10:00:00Z"},{"id":"1002","title":"Remote Designer","slug":"remote-designer-1002","company":{"name":"DesignStudio","slug":"designstudio"},"compensation":{"min":80000,"currency":"USD"},"locations":["Remote"],"remote":true,"description":"Design awesome interfaces.","createdAt":"2026-05-21T14:30:00Z"},{"id":"1003","title":"","company":{"name":"Empty Inc"}}]}}}
</script>
</head>
<body></body>
</html>`

func TestScraper_SiteName(t *testing.T) {
	s := New(nil)
	if got := s.SiteName(); got != model.SiteWellfound {
		t.Errorf("SiteName() = %q, want %q", got, model.SiteWellfound)
	}
}

func TestScraper_Scrape(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(testHTML))
	}))
	defer ts.Close()

	s := NewWithBaseURL(nil, ts.URL)
	jobs, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 25})
	if err != nil {
		t.Fatalf("Scrape() returned error: %v", err)
	}

	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(jobs))
	}

	// Job 0: Software Engineer
	j0 := jobs[0]
	if j0.ID != "wellfound-1001" {
		t.Errorf("job[0].ID = %q, want %q", j0.ID, "wellfound-1001")
	}
	if j0.Title != "Software Engineer" {
		t.Errorf("job[0].Title = %q, want %q", j0.Title, "Software Engineer")
	}
	if j0.CompanyName != "TechCorp" {
		t.Errorf("job[0].CompanyName = %q, want %q", j0.CompanyName, "TechCorp")
	}
	if j0.CompanyLogo != "https://logo.example.com/tc.png" {
		t.Errorf("job[0].CompanyLogo = %q", j0.CompanyLogo)
	}
	if j0.Site != string(model.SiteWellfound) {
		t.Errorf("job[0].Site = %q, want %q", j0.Site, model.SiteWellfound)
	}
	if j0.JobURL != "https://wellfound.com/jobs/software-engineer-1001" {
		t.Errorf("job[0].JobURL = %q", j0.JobURL)
	}
	if j0.Location.City != "San Francisco, CA" {
		t.Errorf("job[0].Location.City = %q", j0.Location.City)
	}
	if j0.IsRemote {
		t.Error("job[0].IsRemote should be false")
	}
	if j0.Compensation == nil {
		t.Fatal("job[0].Compensation is nil")
	}
	if j0.Compensation.MinAmount == nil || *j0.Compensation.MinAmount != 120000 {
		t.Errorf("job[0].Compensation.MinAmount = %v", j0.Compensation.MinAmount)
	}
	if j0.DatePosted == nil {
		t.Error("job[0].DatePosted is nil")
	}

	// Job 1: Remote Designer
	j1 := jobs[1]
	if j1.ID != "wellfound-1002" {
		t.Errorf("job[1].ID = %q, want %q", j1.ID, "wellfound-1002")
	}
	if j1.Title != "Remote Designer" {
		t.Errorf("job[1].Title = %q, want %q", j1.Title, "Remote Designer")
	}
	if j1.CompanyName != "DesignStudio" {
		t.Errorf("job[1].CompanyName = %q, want %q", j1.CompanyName, "DesignStudio")
	}
	if !j1.IsRemote {
		t.Error("job[1].IsRemote should be true")
	}
	if j1.Location.City != "Remote" {
		t.Errorf("job[1].Location.City = %q, want %q", j1.Location.City, "Remote")
	}
	if j1.Compensation == nil || j1.Compensation.MaxAmount != nil {
		t.Errorf("job[1].Compensation should have only min amount")
	}
}

func TestScraper_Scrape_EmptyResponse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, `<html><body>No __NEXT_DATA__ here</body></html>`)
	}))
	defer ts.Close()

	s := NewWithBaseURL(nil, ts.URL)
	_, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 25})
	if err == nil {
		t.Fatal("expected error for missing __NEXT_DATA__, got nil")
	}
}

func TestScraper_Scrape_HTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	s := NewWithBaseURL(nil, ts.URL)
	_, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 25})
	if err == nil {
		t.Fatal("expected error for 500 status, got nil")
	}
}

func TestExtractNextData(t *testing.T) {
	html := `<html><head><script id="__NEXT_DATA__" type="application/json">{"key":"value"}</script></head></html>`
	result := extractNextData(html)
	if result != `{"key":"value"}` {
		t.Errorf("extractNextData = %q, want %q", result, `{"key":"value"}`)
	}
}

func TestExtractNextData_NotFound(t *testing.T) {
	if got := extractNextData("<html></html>"); got != "" {
		t.Errorf("expected empty for no __NEXT_DATA__, got %q", got)
	}
}

func TestMapListing_EmptyTitle(t *testing.T) {
	l := listing{Title: ""}
	if got := mapListing(l); got != nil {
		t.Error("mapListing should return nil for empty title")
	}
}

func TestNewWithBaseURL_EmptyEndpoint(t *testing.T) {
	s1 := NewWithBaseURL(nil, "")
	s2 := New(nil)
	if s1.baseURL != s2.baseURL {
		t.Errorf("empty endpoint should not override base URL")
	}
}

func TestScraper_Scrape_SearchTerm(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify search term is passed in the URL
		if !stringsContains(r.URL.RawQuery, "q=designer") {
			t.Errorf("URL should contain search term, got: %s", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(testHTML))
	}))
	defer ts.Close()

	s := NewWithBaseURL(nil, ts.URL)
	_, err := s.Scrape(context.Background(), model.ScraperInput{
		SearchTerm:    "designer",
		ResultsWanted: 25,
	})
	if err != nil {
		t.Fatalf("Scrape() returned error: %v", err)
	}
}

// Helper
func stringsContains(s, substr string) bool {
	return len(s) >= len(substr) && containsStr(s, substr)
}

func containsStr(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Ensure json.Number works for our test
var _ = json.Number("1")
