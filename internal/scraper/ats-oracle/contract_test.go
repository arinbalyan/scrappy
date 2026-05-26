package oracle

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
	if got := s.SiteName(); got != model.SiteOracle {
		t.Errorf("SiteName() = %q, want %q", got, model.SiteOracle)
	}
}

func TestScraper_Scrape(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := oracleJobsResponse{
			Items: []oracleRequisitionWrapper{
				{
					RequisitionList: []oracleRequisition{
						{
							ID:              "1001",
							Title:           "Software Engineer",
							PrimaryLocation: "Austin, TX, United States",
							PostedDate:      "2025-01-15",
							EmployerName:    "Acme Corp",
							ExternalURLSeo:  "1001",
						},
						{
							ID:              "1002",
							Title:           "Product Manager",
							PrimaryLocation: "Remote",
							PostedDate:      "2025-01-14",
						},
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	s := NewWithAPIURL(nil, ts.URL)
	// Use hyphenated slug format
	result, err := s.Scrape(context.Background(), model.ScraperInput{
		SearchTerm:    "acme-us2",
		ResultsWanted: 25,
	})
	if err != nil {
		t.Fatalf("Scrape() returned error: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(result))
	}

	j1 := result[0]
	if !strings.HasPrefix(j1.ID, "oracle-") {
		t.Errorf("job[0].ID = %q, expected oracle- prefix", j1.ID)
	}
	if j1.Title != "Software Engineer" {
		t.Errorf("job[0].Title = %q", j1.Title)
	}
	if j1.CompanyName != "Acme Corp" {
		t.Errorf("job[0].CompanyName = %q", j1.CompanyName)
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
		t.Fatal("expected error for no tenant, got nil")
	}
}

func TestScraper_Scrape_HTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	s := NewWithAPIURL(nil, ts.URL)
	_, err := s.Scrape(context.Background(), model.ScraperInput{SearchTerm: "acme-us2"})
	if err == nil {
		t.Fatal("expected error for 404, got nil")
	}
}
