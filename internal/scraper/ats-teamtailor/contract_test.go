package teamtailor

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
	if got := s.SiteName(); got != model.SiteTeamTailor {
		t.Errorf("SiteName() = %q, want %q", got, model.SiteTeamTailor)
	}
}

func TestScraper_Scrape(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := ttResponse{
			Data: []ttJob{
				{
					ID:   "1",
					Type: "jobs",
					Attributes: &ttAttrs{
						Title:   "Software Engineer",
						Body:    "<p>Build cool stuff</p>",
						City:    "San Francisco",
						Region:  "CA",
						Country: "USA",
						Remote:  false,
						CreatedAt: "2025-01-15T00:00:00Z",
					},
					Links: &struct {
						Self         string `json:"self,omitempty"`
						CareersiteURL string `json:"careersite-url,omitempty"`
					}{CareersiteURL: "https://career.acme.com/jobs/1"},
				},
				{
					ID:   "2",
					Type: "jobs",
					Attributes: &ttAttrs{
						Title:   "Product Manager",
						Body:    "<p>Manage products</p>",
						Remote:  true,
						CreatedAt: "2025-01-14T00:00:00Z",
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
	if !strings.HasPrefix(j1.ID, "teamtailor-") {
		t.Errorf("job[0].ID = %q, expected teamtailor- prefix", j1.ID)
	}
	if j1.Title != "Software Engineer" {
		t.Errorf("job[0].Title = %q", j1.Title)
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
