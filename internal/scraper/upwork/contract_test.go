package upwork

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/arinbalyan/scrappy/internal/model"
)

func TestScraper_SiteName(t *testing.T) {
	s := New(nil)
	if got := s.SiteName(); got != model.SiteUpwork {
		t.Errorf("SiteName() = %q, want %q", got, model.SiteUpwork)
	}
}

func TestScraper_Scrape_MissingCredentials(t *testing.T) {
	s := New(nil)
	_, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 25})
	if err == nil {
		t.Fatal("expected error when credentials are missing, got nil")
	}
}

func TestScraper_Scrape_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		resp := graphQLResponse{
			Data: &graphQLData{
				MarketplaceJobPostings: &jobPostingsConnection{
					Edges: []jobEdge{
						{
							Node: &jobNode{
								ID:              "job_123",
								Ciphertext:      "abcdef123",
								Title:           "Go Developer needed",
								Description:     "We need a Go expert for a 3-month project.",
								CreatedDateTime: "2026-05-20T10:00:00Z",
								Engagement:      "Full-time",
								Amount:          &money{Amount: "8000", CurrencyCode: "USD"},
								Skills:          []skill{{Name: "Go"}, {Name: "PostgreSQL"}},
								ContractorTier:  "EXPERT",
							},
						},
						{
							Node: &jobNode{
								ID:         "job_456",
								Title:      "React Developer",
								Description: "Build UIs with React and TypeScript.",
								CreatedDateTime: "2026-05-21T14:30:00Z",
								WeeklyBudget: &money{Amount: "1000", CurrencyCode: "USD"},
							},
						},
					},
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	s := NewWithToken(nil, "test-token")
	s.graphqlURL = ts.URL

	result, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 25})
	if err != nil {
		t.Fatalf("Scrape() returned error: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(result))
	}

	j0 := result[0]
	if j0.ID != "upwork-job_123" {
		t.Errorf("job[0].ID = %q, want %q", j0.ID, "upwork-job_123")
	}
	if j0.Title != "Go Developer needed" {
		t.Errorf("job[0].Title = %q, want %q", j0.Title, "Go Developer needed")
	}
	if j0.Site != string(model.SiteUpwork) {
		t.Errorf("job[0].Site = %q, want %q", j0.Site, model.SiteUpwork)
	}
	if j0.DatePosted == nil {
		t.Error("job[0].DatePosted is nil")
	}

	j1 := result[1]
	if j1.Title != "React Developer" {
		t.Errorf("job[1].Title = %q, want %q", j1.Title, "React Developer")
	}
}

func TestScraper_Scrape_HTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	s := NewWithToken(nil, "test-token")
	s.graphqlURL = ts.URL

	_, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 25})
	if err == nil {
		t.Fatal("expected error for 500 status, got nil")
	}
}

func TestScraper_Scrape_EmptyResponse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data":{"marketplaceJobPostings":{"edges":[]}}}`))
	}))
	defer ts.Close()

	s := NewWithToken(nil, "test-token")
	s.graphqlURL = ts.URL

	_, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 25})
	if err == nil {
		t.Fatal("expected error for empty results, got nil")
	}
}

func TestNewWithGraphQLURL_EmptyEndpoint(t *testing.T) {
	s1 := NewWithGraphQLURL(nil, "")
	s2 := New(nil)
	if s1.graphqlURL != s2.graphqlURL {
		t.Errorf("empty endpoint should not override GraphQL URL")
	}
}
