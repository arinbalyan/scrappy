package functionalworks

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/arinbalyan/scrappy/internal/model"
)

func TestScraper_SiteName(t *testing.T) {
	s := New(nil)
	if got := s.SiteName(); got != model.SiteFunctionalWorks {
		t.Errorf("SiteName() = %q, want %q", got, model.SiteFunctionalWorks)
	}
}

func TestScraper_Scrape(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify it's a POST with GraphQL query
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")

		resp := graphQLResponse{
			Data: &graphQLData{
				Jobs: []graphQLJob{
					{
						Title: "Senior Haskell Engineer",
						Company: &graphQLCompany{Name: "FuncCorp"},
						Location: &graphQLLocation{City: "London", Country: "UK"},
						Remote: true,
						Remuneration: &graphQLRemun{
							TimePeriod: "Yearly",
							Currency:   "GBP",
							Min:        fptr(80000),
							Max:        fptr(120000),
						},
						Slug:           "senior-haskell-engineer",
						FirstPublished: "2026-05-15T10:00:00Z",
						DescriptionHTML: "<p>Build functional systems.</p>",
						Tags:           []graphQLTag{{Label: "haskell"}, {Label: "purescript"}},
					},
					{
						Title: "Clojure Developer",
						Company: &graphQLCompany{Name: "DataFunc"},
						Remote: false,
						Slug:   "clojure-developer",
						Tags:   []graphQLTag{{Label: "clojure"}},
					},
					{
						Title: "",
						Slug:   "empty-title",
					},
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	s := NewWithAPIURL(nil, ts.URL)
	result, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 25})
	if err != nil {
		t.Fatalf("Scrape() returned error: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(result))
	}

	// Check first job
	j1 := result[0]
	if j1.ID != "functionalworks-senior-haskell-engineer" {
		t.Errorf("job[0].ID = %q, want %q", j1.ID, "functionalworks-senior-haskell-engineer")
	}
	if j1.Title != "Senior Haskell Engineer" {
		t.Errorf("job[0].Title = %q, want %q", j1.Title, "Senior Haskell Engineer")
	}
	if j1.CompanyName != "FuncCorp" {
		t.Errorf("job[0].CompanyName = %q, want %q", j1.CompanyName, "FuncCorp")
	}
	if j1.Site != string(model.SiteFunctionalWorks) {
		t.Errorf("job[0].Site = %q, want %q", j1.Site, model.SiteFunctionalWorks)
	}
	if !j1.IsRemote {
		t.Error("job[0].IsRemote should be true")
	}
	if j1.Compensation == nil {
		t.Fatal("job[0].Compensation is nil")
	}
	if j1.Compensation.Currency != "GBP" {
		t.Errorf("job[0].Compensation.Currency = %q", j1.Compensation.Currency)
	}
	if j1.Compensation.MinAmount == nil || math.Abs(*j1.Compensation.MinAmount-80000) > 0.01 {
		t.Errorf("job[0].Compensation.MinAmount = %v", j1.Compensation.MinAmount)
	}
	if j1.DatePosted == nil {
		t.Error("job[0].DatePosted is nil")
	}
	if !strings.Contains(j1.Description, "Tags: haskell, purescript") {
		t.Errorf("job[0].Description should contain tags, got: %q", j1.Description)
	}
	if !strings.Contains(j1.Description, "Build functional systems") {
		t.Errorf("job[0].Description should contain description HTML, got: %q", j1.Description)
	}
	if j1.Location.City != "London" {
		t.Errorf("job[0].Location.City = %q", j1.Location.City)
	}

	// Check second job (no location, no compensation)
	j2 := result[1]
	if j2.ID != "functionalworks-clojure-developer" {
		t.Errorf("job[1].ID = %q, want %q", j2.ID, "functionalworks-clojure-developer")
	}
	if j2.CompanyName != "DataFunc" {
		t.Errorf("job[1].CompanyName = %q, want %q", j2.CompanyName, "DataFunc")
	}
	if j2.IsRemote {
		t.Error("job[1].IsRemote should be false")
	}
	if j2.Compensation != nil {
		t.Error("job[1].Compensation should be nil")
	}
}

func TestScraper_Scrape_SearchTerm(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := graphQLResponse{
			Data: &graphQLData{
				Jobs: []graphQLJob{
					{
						Title: "Haskell Developer",
						Slug:  "haskell-dev",
					},
					{
						Title: "Elixir Developer",
						Slug:  "elixir-dev",
					},
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	s := NewWithAPIURL(nil, ts.URL)
	result, err := s.Scrape(context.Background(), model.ScraperInput{
		SearchTerm:    "elixir",
		ResultsWanted: 25,
	})
	if err != nil {
		t.Fatalf("Scrape() returned error: %v", err)
	}

	if len(result) != 1 {
		t.Fatalf("expected 1 job matching 'elixir', got %d", len(result))
	}
	if result[0].Title != "Elixir Developer" {
		t.Errorf("job[0].Title = %q, want %q", result[0].Title, "Elixir Developer")
	}
}

func TestScraper_Scrape_HTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	s := NewWithAPIURL(nil, ts.URL)
	_, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 25})
	if err == nil {
		t.Fatal("expected error for 500 status, got nil")
	}
}

func TestScraper_Scrape_EmptyBody(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data":{"jobs":[]}}`))
	}))
	defer ts.Close()

	s := NewWithAPIURL(nil, ts.URL)
	_, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 25})
	if err == nil {
		t.Fatal("expected error for empty response, got nil")
	}
}

func TestNewWithAPIURL_EmptyEndpoint(t *testing.T) {
	s1 := NewWithAPIURL(nil, "")
	s2 := New(nil)
	if s1.apiURL != s2.apiURL {
		t.Errorf("empty endpoint should not override API URL")
	}
}

func fptr(v float64) *float64 { return &v }

// suppress unused import warning
var _ = fmt.Sprintf
