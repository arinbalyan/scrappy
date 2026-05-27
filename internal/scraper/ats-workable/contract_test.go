package workable

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/arinbalyan/scrappy/internal/model"
)

func TestScraper_SiteName(t *testing.T) {
	s := New(nil)
	if got := s.SiteName(); got != model.SiteWorkable {
		t.Errorf("SiteName() = %q, want %q", got, model.SiteWorkable)
	}
}

func TestScraper_Scrape(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := wbResponse{
			Jobs: []wbJob{
				{
					Title:          "Software Engineer",
					Shortcode:      "abc123",
					EmploymentType: "Full-Time",
					Telecommuting:  false,
					Department:     "Engineering",
					URL:            "https://apply.workable.com/acme/j/abc123",
					PublishedOn:    "2025-01-15",
					Locations: []wbLocation{
						{City: "San Francisco", Region: "CA", Country: "USA"},
					},
				},
				{
					Title:         "Product Manager",
					Shortcode:     "def456",
					Telecommuting: true,
					Shortlink:     "https://apply.workable.com/acme/j/def456",
					PublishedOn:   "2025-01-14",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
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
	if !strings.HasPrefix(j1.ID, "workable-") {
		t.Errorf("job[0].ID = %q, expected workable- prefix", j1.ID)
	}
	if j1.Title != "Software Engineer" {
		t.Errorf("job[0].Title = %q", j1.Title)
	}
	if j1.Department != "Engineering" {
		t.Errorf("job[0].Department = %q", j1.Department)
	}
	if j1.Location.City != "San Francisco" {
		t.Errorf("job[0].Location.City = %q", j1.Location.City)
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
