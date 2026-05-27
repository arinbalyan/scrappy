package personio

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/arinbalyan/scrappy/internal/model"
)

const testXML = `<?xml version="1.0" encoding="UTF-8"?>
<workzag-jobs>
  <position>
    <id>101</id>
    <name>Software Engineer</name>
    <office>Munich</office>
    <department>Engineering</department>
    <employmentType>fulltime</employmentType>
    <seniority>Senior</seniority>
    <keywords>go,python,kubernetes</keywords>
    <createdAt>2025-01-15</createdAt>
    <jobDescriptions>
      <jobDescription>
        <name>description</name>
        <value>&lt;p&gt;Build cool stuff&lt;/p&gt;</value>
      </jobDescription>
    </jobDescriptions>
  </position>
  <position>
    <id>102</id>
    <name>Product Manager</name>
    <office>Remote</office>
    <createdAt>2025-01-14</createdAt>
    <jobDescriptions/>
  </position>
</workzag-jobs>`

func TestScraper_SiteName(t *testing.T) {
	s := New(nil)
	if got := s.SiteName(); got != model.SitePersonio {
		t.Errorf("SiteName() = %q, want %q", got, model.SitePersonio)
	}
}

func TestScraper_Scrape(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.Write([]byte(testXML))
	}))
	defer ts.Close()

	s := NewWithAPIURL(nil, ts.URL)
	result, err := s.Scrape(context.Background(), model.ScraperInput{
		SearchTerm:    "acme",
		ResultsWanted: 25,
	})
	if err != nil {
		t.Fatalf("Scrape() returned error: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(result))
	}

	j1 := result[0]
	if !strings.HasPrefix(j1.ID, "personio-") {
		t.Errorf("job[0].ID = %q, expected personio- prefix", j1.ID)
	}
	if j1.Title != "Software Engineer" {
		t.Errorf("job[0].Title = %q", j1.Title)
	}
	if j1.Department != "Engineering" {
		t.Errorf("job[0].Department = %q", j1.Department)
	}
	if j1.Location.City != "Munich" {
		t.Errorf("job[0].Location.City = %q", j1.Location.City)
	}
	if len(j1.Skills) != 3 {
		t.Errorf("job[0].Skills = %v, expected 3 skills", j1.Skills)
	}

	j2 := result[1]
	if j2.Title != "Product Manager" {
		t.Errorf("job[1].Title = %q", j2.Title)
	}
}

func TestScraper_Scrape_NoSeeds(t *testing.T) {
	s := New(nil)
	_, err := s.Scrape(context.Background(), model.ScraperInput{})
	if err == nil {
		t.Fatal("expected error for no seeds, got nil")
	}
}

func TestScraper_Scrape_HTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	s := NewWithAPIURL(nil, ts.URL)
	_, err := s.Scrape(context.Background(), model.ScraperInput{SearchTerm: "acme"})
	if err == nil {
		t.Fatal("expected error for 404, got nil")
	}
}

func TestScraper_Scrape_EmptyXML(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.Write([]byte(`<?xml version="1.0"?><workzag-jobs></workzag-jobs>`))
	}))
	defer ts.Close()

	s := NewWithAPIURL(nil, ts.URL)
	_, err := s.Scrape(context.Background(), model.ScraperInput{SearchTerm: "acme"})
	if err == nil {
		t.Fatal("expected error for empty positions, got nil")
	}
}
