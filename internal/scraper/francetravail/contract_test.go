package francetravail

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
)

func TestScraper_SiteName(t *testing.T) {
	s := New(nil)
	if got := s.SiteName(); got != model.SiteFranceTravail {
		t.Errorf("SiteName() = %q, want %q", got, model.SiteFranceTravail)
	}
}

func TestScraper_Scrape_MissingCredentials(t *testing.T) {
	// Unset env vars if set — we must verify the error path
	t.Setenv("FRANCETRAVAIL_CLIENT_ID", "")
	t.Setenv("FRANCETRAVAIL_CLIENT_SECRET", "")

	s := New(nil)
	_, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 25})
	if err == nil {
		t.Fatal("expected error when credentials are missing")
	}
	if !strings.Contains(err.Error(), "CLIENT_ID") {
		t.Errorf("error should mention credentials, got: %v", err)
	}
}

func TestScraper_Scrape_Success(t *testing.T) {
	// Mock server that handles both token and search requests
	var tokenRequestCount int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "access_token") || strings.Contains(r.URL.RawQuery, "grant_type") || strings.Contains(r.URL.Path, "connexion") {
			tokenRequestCount++
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(tokenResponse{
				AccessToken: "test-token-123",
				ExpiresIn:   3600,
			})
			return
		}

		// Search request
		w.Header().Set("Content-Type", "application/json")
		resp := searchResponse{
			Resultats: []offer{
				{
					ID:          "12345",
					Intitule:    "Developpeur Go",
					Description: "Develop Go applications",
					DateCreation: "2026-05-15T10:00:00Z",
					LieuTravail: &lieuTravail{Libelle: "Paris"},
					Entreprise:  &entreprise{Nom: "TechCorp France"},
					OrigineOffre: &origineOffre{URLOrigine: "https://example.com/job/12345"},
				},
				{
					ID:       "67890",
					Intitule: "Ingenieur DevOps",
					DateCreation: "2026-05-16T14:30:00Z",
					LieuTravail: &lieuTravail{Libelle: "Lyon"},
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	s := New(nil)
	s.clientID = "test-client-id"
	s.clientSecret = "test-client-secret"
	s.apiURL = ts.URL

	exp := time.Now().Add(1 * time.Hour)
	s.mu.Lock()
	s.tokenExpires = &exp
	s.accessToken = "test-token"
	s.mu.Unlock()

	result, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 25})
	if err != nil {
		t.Fatalf("Scrape() returned error: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(result))
	}

	j1 := result[0]
	if j1.ID != "francetravail-12345" {
		t.Errorf("job[0].ID = %q, want %q", j1.ID, "francetravail-12345")
	}
	if j1.Title != "Developpeur Go" {
		t.Errorf("job[0].Title = %q, want %q", j1.Title, "Developpeur Go")
	}
	if j1.CompanyName != "TechCorp France" {
		t.Errorf("job[0].CompanyName = %q, want %q", j1.CompanyName, "TechCorp France")
	}
	if j1.Site != string(model.SiteFranceTravail) {
		t.Errorf("job[0].Site = %q, want %q", j1.Site, model.SiteFranceTravail)
	}
	if j1.JobURL != "https://example.com/job/12345" {
		t.Errorf("job[0].JobURL = %q", j1.JobURL)
	}
	if j1.Location.City != "Paris" {
		t.Errorf("job[0].Location.City = %q", j1.Location.City)
	}
	if j1.DatePosted == nil {
		t.Error("job[0].DatePosted is nil")
	}

	j2 := result[1]
	if j2.ID != "francetravail-67890" {
		t.Errorf("job[1].ID = %q, want %q", j2.ID, "francetravail-67890")
	}
	if j2.Location.City != "Lyon" {
		t.Errorf("job[1].Location.City = %q", j2.Location.City)
	}
	// Should use fallback URL
	if !strings.Contains(j2.JobURL, "francetravail.fr/offres/recherche/detail/67890") {
		t.Errorf("job[1].JobURL should be fallback, got: %s", j2.JobURL)
	}
}

func TestScraper_Scrape_HTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	s := New(nil)
	s.clientID = "test"
	exp2 := time.Now().Add(1 * time.Hour)
	s.clientSecret = "test"
	s.apiURL = ts.URL
	s.mu.Lock()
	s.tokenExpires = &exp2
	s.accessToken = "test-token"
	s.mu.Unlock()

	// The token request will fail first
	_, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 25})
	if err == nil {
		t.Fatal("expected error for failed token request, got nil")
	}
}

func TestScraper_Scrape_EmptyBody(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, `{"resultats":[]}`)
	}))
	defer ts.Close()

	s := New(nil)
	s.clientID = "test"
	s.clientSecret = "test"
	s.apiURL = ts.URL
	s.accessToken = "test-token"
	s.mu.Lock()
	exp := time.Now().Add(1 * time.Hour)
	s.tokenExpires = &exp
	s.mu.Unlock()

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
