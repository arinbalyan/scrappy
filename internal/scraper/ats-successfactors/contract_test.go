package successfactors

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
	if got := s.SiteName(); got != model.SiteSuccessFactors {
		t.Errorf("SiteName() = %q, want %q", got, model.SiteSuccessFactors)
	}
}

func TestScraper_Scrape(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := sfODataResponse{
			D: &sfODataWrapper{
				Results: []sfJobPosting{
					{
						JobReqID:        "1001",
						JobTitle:        "Software Engineer",
						JobDescription:  "Build cool stuff",
						Department:      "Engineering",
						PostingStartDate: "2025-01-15T00:00:00Z",
						EmploymentType:  "Full-Time",
						CompanyName:     "Acme Corp",
						LocationObj: &struct {
							City    string `json:"city,omitempty"`
							State   string `json:"state,omitempty"`
							Country string `json:"country,omitempty"`
						}{City: "San Francisco", State: "CA", Country: "USA"},
					},
					{
						JobReqID:        "1002",
						JobTitle:        "Product Manager",
						PostingStartDate: "2025-01-14T00:00:00Z",
						LocationObj: &struct {
							City    string `json:"city,omitempty"`
							State   string `json:"state,omitempty"`
							Country string `json:"country,omitempty"`
						}{City: "Remote"},
					},
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
	if !strings.HasPrefix(j1.ID, "sf-") {
		t.Errorf("job[0].ID = %q, expected sf- prefix", j1.ID)
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
