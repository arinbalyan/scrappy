package trakstar

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
	if got := s.SiteName(); got != model.SiteTrakstar {
		t.Errorf("SiteName() = %q, want %q", got, model.SiteTrakstar)
	}
}

func TestScraper_Scrape(t *testing.T) {
	remote := true
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jobs := []trakstarJob{
			{
				ID:             1,
				Title:          "Software Engineer",
				Description:    "<p>Build cool stuff</p>",
				Department:     "Engineering",
				City:           "San Francisco",
				State:          "CA",
				EmploymentType: "Full-Time",
				CreatedAt:      "2025-01-15T00:00:00Z",
				Remote:         &remote,
				SalaryMin:      100000,
				SalaryMax:      150000,
				SalaryCurrency:  "USD",
			},
			{
				ID:        2,
				Title:     "Product Manager",
				City:      "Remote",
				CreatedAt: "2025-01-14T00:00:00Z",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jobs)
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
	if !strings.HasPrefix(j1.ID, "trakstar-") {
		t.Errorf("job[0].ID = %q, expected trakstar- prefix", j1.ID)
	}
	if j1.Title != "Software Engineer" {
		t.Errorf("job[0].Title = %q", j1.Title)
	}
	if j1.Department != "Engineering" {
		t.Errorf("job[0].Department = %q", j1.Department)
	}
	if j1.Compensation == nil {
		t.Error("job[0].Compensation is nil")
	}
	if !j1.IsRemote {
		t.Error("job[0].IsRemote should be true")
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
