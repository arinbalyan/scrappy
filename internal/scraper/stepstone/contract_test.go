package stepstone

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/arinbalyan/scrappy/internal/model"
)

const testHTML = `<!DOCTYPE html>
<html>
<head><title>StepStone Jobs</title></head>
<body>
  <article data-testid="job-item">
    <h2><a href="/stellenangebote--senior-go-developer--12345">Senior Go Developer</a></h2>
    <div class="res-company-name">TechCorp GmbH</div>
    <div class="res-location">Berlin</div>
  </article>
  <article data-testid="job-item">
    <h2><a href="/jobs--frontend-engineer--67890">Frontend Engineer</a></h2>
    <div class="res-company-name">WebStartup AG</div>
    <div class="res-location">Munich</div>
  </article>
  <script type="application/ld+json">
  {
    "@context": "https://schema.org",
    "@type": "JobPosting",
    "title": "Senior Go Developer",
    "description": "<p>Build Go microservices.</p>",
    "datePosted": "2026-05-15T10:00:00Z",
    "baseSalary": {
      "currency": "EUR",
      "value": {"minValue": 80000, "maxValue": 110000}
    },
    "hiringOrganization": {"name": "TechCorp GmbH"},
    "employmentType": "FULL_TIME"
  }
  </script>
</body>
</html>`

func TestScraper_SiteName(t *testing.T) {
	s := New(nil)
	if got := s.SiteName(); got != model.SiteStepStone {
		t.Errorf("SiteName() = %q, want %q", got, model.SiteStepStone)
	}
}

func TestScraper_Scrape(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(testHTML))
	}))
	defer ts.Close()

	// Override domain to point to test server
	s := NewWithDomain(nil, ts.URL)
	jobs, err := s.Scrape(context.Background(), model.ScraperInput{
		SearchTerm:    "developer",
		ResultsWanted: 10,
	})
	if err != nil {
		t.Fatalf("Scrape() returned error: %v", err)
	}

	if len(jobs) < 1 {
		t.Fatalf("expected at least 1 job, got %d", len(jobs))
	}

	// Check first job
	j1 := jobs[0]
	if j1.Title != "Senior Go Developer" {
		t.Errorf("job[0].Title = %q, want %q", j1.Title, "Senior Go Developer")
	}
	if !stringsContains(j1.JobURL, "/stellenangebote--senior-go-developer--12345") {
		t.Errorf("job[0].JobURL = %q, should contain the job path", j1.JobURL)
	}
	if j1.Site != string(model.SiteStepStone) {
		t.Errorf("job[0].Site = %q, want %q", j1.Site, model.SiteStepStone)
	}

	// Description should be enriched from JSON-LD
	if j1.Description == "" {
		t.Error("job[0].Description should be populated from JSON-LD")
	}

	// Compensation should be enriched from JSON-LD
	if j1.Compensation != nil && j1.Compensation.Currency != "EUR" {
		t.Errorf("job[0].Compensation.Currency = %q, want %q", j1.Compensation.Currency, "EUR")
	}

	// DatePosted should be enriched from JSON-LD
	if j1.DatePosted == nil {
		t.Error("job[0].DatePosted should be populated from JSON-LD")
	}
}

func TestScraper_Scrape_EmptySearchTerm(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(testHTML))
	}))
	defer ts.Close()

	s := NewWithDomain(nil, ts.URL)
	// Empty search term should default to "developer"
	jobs, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 10})
	if err != nil {
		t.Fatalf("Scrape() returned error: %v", err)
	}

	if len(jobs) < 1 {
		t.Fatal("expected at least 1 job")
	}
}

func TestScraper_Scrape_HTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	s := NewWithDomain(nil, ts.URL)
	_, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 10})
	if err == nil {
		t.Fatal("expected error for 500 status, got nil")
	}
}

func TestScraper_Scrape_EmptyBody(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`<html><body>No jobs found</body></html>`))
	}))
	defer ts.Close()

	s := NewWithDomain(nil, ts.URL)
	_, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 10})
	if err == nil {
		t.Fatal("expected error for empty results, got nil")
	}
}

func TestNewWithDomain_EmptyDomain(t *testing.T) {
	s1 := NewWithDomain(nil, "")
	s2 := New(nil)
	if s1.domain != s2.domain {
		t.Errorf("empty domain should not override default domain")
	}
}

// stringsContains is a helper for substring checks.
func stringsContains(s, substr string) bool {
	return len(s) >= len(substr) && containsStr(s, substr)
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
