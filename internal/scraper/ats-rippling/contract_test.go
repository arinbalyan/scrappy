package rippling

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/arinbalyan/scrappy/internal/model"
)

func TestScraper_SiteName(t *testing.T) {
	s := New(nil)
	if got := s.SiteName(); got != model.SiteRippling {
		t.Errorf("SiteName() = %q, want %q", got, model.SiteRippling)
	}
}

func buildTestHTML(t *testing.T) string {
	t.Helper()
	// Build the next data JSON string programmatically
	items := `[{"uuid":"abc-123","title":"Software Engineer","companyName":"Acme Corp","locations":[{"city":"San Francisco","state":"CA","country":"USA","workplaceType":"office"}],"description":{"role":"Build cool stuff"},"department":{"name":"Engineering"},"employmentType":{"label":"Full-Time"},"createdOn":"2025-01-15T00:00:00Z"},{"uuid":"def-456","title":"Product Manager","companyName":"Acme Corp","locations":[{"city":"Remote","state":"CA","country":"USA","workplaceType":"remote"}],"description":{"role":"Manage products"},"createdOn":"2025-01-14T00:00:00Z"}]`

	// Verify the next data JSON is valid
	data := `{"props":{"pageProps":{"dehydratedState":{"queries":[{"state":{"data":{"items":` + items + `}}}]}}}}`
	return `<html><body><script id="__NEXT_DATA__" type="application/json">` + data + `</script></body></html>`
}

func TestScraper_Scrape(t *testing.T) {
	html := buildTestHTML(t)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(html))
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
	if !strings.HasPrefix(j1.ID, "rippling-") {
		t.Errorf("job[0].ID = %q, expected rippling- prefix", j1.ID)
	}
	if j1.Title != "Software Engineer" {
		t.Errorf("job[0].Title = %q", j1.Title)
	}
	if j1.CompanyName != "Acme Corp" {
		t.Errorf("job[0].CompanyName = %q", j1.CompanyName)
	}
	if j1.Department != "Engineering" {
		t.Errorf("job[0].Department = %q", j1.Department)
	}

	j2 := result[1]
	if j2.Title != "Product Manager" {
		t.Errorf("job[1].Title = %q", j2.Title)
	}
	if !j2.IsRemote {
		t.Error("job[1].IsRemote should be true")
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

func TestScraper_Scrape_NoNextData(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte("<html><body>no next data</body></html>"))
	}))
	defer ts.Close()

	s := NewWithAPIURL(nil, ts.URL)
	_, err := s.Scrape(context.Background(), model.ScraperInput{SearchTerm: "acme"})
	if err == nil {
		t.Fatal("expected error for missing __NEXT_DATA__, got nil")
	}
}
