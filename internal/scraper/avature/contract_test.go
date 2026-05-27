package avature

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
	if got := s.SiteName(); got != model.SiteAvature {
		t.Errorf("SiteName() = %q, want %q", got, model.SiteAvature)
	}
}

func TestScraper_Scrape(t *testing.T) {
	// Make title matches align — each link gets a corresponding title via title attribute
	html := `<html><body>
		<div class="job-item">
			<a href="/careers/JobDetail/Software-Engineer-123" title="Software Engineer">Apply</a>
			<span class="location">San Francisco, CA</span>
		</div>
		<div class="job-item">
			<a href="/careers/JobDetail/Product-Manager-456" title="Product Manager">Apply</a>
			<span class="location">Remote, US</span>
		</div>
	</body></html>`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(html))
	}))
	defer ts.Close()

	// Override the base URL by using a seed that creates the right URL
	// We'll use httptest server URL directly by overriding fetchAllPages behavior
	// Instead, use the custom approach: point to our test server
	_ = ts // use ts.URL below

	// Directly test parseListings
	jobs := parseListings(html, ts.URL, "Acme", 0, 25)
	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(jobs))
	}

	j1 := jobs[0]
	if !strings.HasPrefix(j1.ID, "avature-") {
		t.Errorf("job[0].ID = %q, expected avature- prefix", j1.ID)
	}
	if j1.Title != "Software Engineer" {
		t.Errorf("job[0].Title = %q", j1.Title)
	}
	if j1.Location.City != "San Francisco, CA" {
		t.Errorf("job[0].Location.City = %q", j1.Location.City)
	}

	j2 := jobs[1]
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

func TestScraper_Scrape_EmptyPage(t *testing.T) {
	html := `<html><body>No jobs available</body></html>`
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(html))
	}))
	defer ts.Close()

	jobs := parseListings(html, ts.URL, "Acme", 0, 25)
	if len(jobs) != 0 {
		t.Errorf("expected 0 jobs, got %d", len(jobs))
	}
}

func TestScraper_fetchAllPages_HTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	s := New(nil)
	_, err := s.fetchAllPages(context.Background(), ts.URL, "Acme", 25)
	if err == nil {
		t.Fatal("expected error for 404 status, got nil")
	}
}
