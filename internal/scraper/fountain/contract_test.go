package fountain

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
	if got := s.SiteName(); got != model.SiteFountain {
		t.Errorf("SiteName() = %q, want %q", got, model.SiteFountain)
	}
}

func f64(v float64) *float64 { return &v }
func bptr(v bool) *bool      { return &v }

func TestScraper_Scrape(t *testing.T) {
	isRemote := true
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		resp := fountainResponse{
			Openings: []fountainOpening{
				{
					ID:          "open-1",
					Title:       "Software Engineer",
					Description: "<p>Build cool stuff</p>",
					Department:  "Engineering",
					IsRemote:    &isRemote,
					URL:         "https://fountain.com/jobs/1",
					CreatedAt:   "2025-01-15T00:00:00Z",
					Compensation: &fountainCompensation{
						MinAmount: f64(80.0),
						MaxAmount: f64(120.0),
						Currency:  "USD",
						Interval:  "hourly",
					},
					Location: &fountainLocation{
						City:    "San Francisco",
						State:   "CA",
						Country: "US",
					},
				},
				{
					ID:             "open-2",
					Title:          "Product Manager",
					Description:    "Manage products",
					LocationString: "Remote",
					CreatedAt:      "2025-01-14T00:00:00Z",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	s := NewWithAPIURL(nil, ts.URL)
	result, err := s.Scrape(context.Background(), model.ScraperInput{
		SearchTerm:    "test-api-key",
		ResultsWanted: 25,
	})
	if err != nil {
		t.Fatalf("Scrape() returned error: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(result))
	}

	j1 := result[0]
	if !strings.HasPrefix(j1.ID, "fountain-") {
		t.Errorf("job[0].ID = %q, expected fountain- prefix", j1.ID)
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
	if !j1.IsRemote {
		t.Error("job[0].IsRemote should be true")
	}
	if j1.Compensation == nil {
		t.Fatal("job[0].Compensation is nil")
	}
	if j1.DatePosted == nil {
		t.Error("job[0].DatePosted is nil")
	}

	j2 := result[1]
	if j2.Title != "Product Manager" {
		t.Errorf("job[1].Title = %q", j2.Title)
	}
	if j2.Location.City != "Remote" {
		t.Errorf("job[1].Location.City = %q", j2.Location.City)
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
	_, err := s.Scrape(context.Background(), model.ScraperInput{SearchTerm: "key"})
	if err == nil {
		t.Fatal("expected error for 404 status, got nil")
	}
}

func TestScraper_Scrape_EmptyBody(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(fountainResponse{Openings: []fountainOpening{}})
	}))
	defer ts.Close()

	s := NewWithAPIURL(nil, ts.URL)
	_, err := s.Scrape(context.Background(), model.ScraperInput{SearchTerm: "key"})
	if err == nil {
		t.Fatal("expected error for empty response, got nil")
	}
}
