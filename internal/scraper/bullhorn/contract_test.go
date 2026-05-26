package bullhorn

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
	if got := s.SiteName(); got != model.SiteBullhorn {
		t.Errorf("SiteName() = %q, want %q", got, model.SiteBullhorn)
	}
}

func TestScraper_Scrape(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := bullhornSearchResponse{
			Data: []bullhornJobOrder{
				{
					ID:                1001,
					Title:             "Software Engineer",
					PublicDescription: "<p>Build cool stuff</p>",
					Salary:            f64(120000.0),
					SalaryUnit:        "Per Year",
					EmploymentType:    "Full-Time",
					Address: &bullhornAddress{
						City:    "San Francisco",
						State:   "CA",
						Country: "US",
					},
					Categories: &bullhornCategories{
						Data: []bullhornCategory{
							{ID: 1, Name: "Engineering"},
						},
					},
					DateAdded: i64ptr(1736899200000), // 2025-01-15
				},
				{
					ID:         1002,
					Title:      "Product Manager",
					Salary:     f64(130000.0),
					SalaryUnit: "Per Year",
				},
			},
			Total: 2,
			Count: 2,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	s := NewWithAPIURL(nil, ts.URL)
	result, err := s.Scrape(context.Background(), model.ScraperInput{
		SearchTerm:    "91:testcorp",
		ResultsWanted: 25,
	})
	if err != nil {
		t.Fatalf("Scrape() returned error: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(result))
	}

	j1 := result[0]
	if !strings.HasPrefix(j1.ID, "bullhorn-") {
		t.Errorf("job[0].ID = %q, expected bullhorn- prefix", j1.ID)
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
	if j1.Compensation == nil {
		t.Fatal("job[0].Compensation is nil")
	}
	if j1.Compensation.Currency != "USD" {
		t.Errorf("job[0].Compensation.Currency = %q", j1.Compensation.Currency)
	}
	if j1.DatePosted == nil {
		t.Error("job[0].DatePosted is nil")
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

func TestParseSlug_Invalid(t *testing.T) {
	_, _, err := parseSlug("invalid")
	if err == nil {
		t.Fatal("expected error for missing colon")
	}
}

func TestParseSlug_Valid(t *testing.T) {
	cls, ct, err := parseSlug("91:abc123def")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cls != "91" {
		t.Errorf("cls = %q, want %q", cls, "91")
	}
	if ct != "abc123def" {
		t.Errorf("corpToken = %q, want %q", ct, "abc123def")
	}
}

func TestScraper_Scrape_HTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	s := NewWithAPIURL(nil, ts.URL)
	_, err := s.Scrape(context.Background(), model.ScraperInput{SearchTerm: "91:testcorp"})
	if err == nil {
		t.Fatal("expected error for 404 status, got nil")
	}
}

func TestScraper_Scrape_EmptyBody(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(bullhornSearchResponse{Data: []bullhornJobOrder{}, Total: 0, Count: 0})
	}))
	defer ts.Close()

	s := NewWithAPIURL(nil, ts.URL)
	_, err := s.Scrape(context.Background(), model.ScraperInput{SearchTerm: "91:testcorp"})
	if err == nil {
		t.Fatal("expected error for empty response, got nil")
	}
}

func f64(v float64) *float64 { return &v }
func i64ptr(v int64) *int64 { return &v }
