package arbeitsagentur_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/arinbalyan/scrappy/internal/model"
	sut "github.com/arinbalyan/scrappy/internal/scraper/arbeitsagentur"
)

func TestArbeitsagenturParsesJobs(t *testing.T) {
	apiKey := "test-key"
	json := `{"stellenangebote":[{"refnr":"REF-001","titel":"Senior Developer","arbeitgeber":"TechCorp","beruf":"Softwareentwicklung","homeOffice":false,"externeUrl":"https://example.com/jobs/1","aktuelleVeroeffentlichungsdatum":"2025-03-15T00:00:00.000Z","arbeitsort":{"ort":"Berlin","plz":"10115","region":"Berlin","land":"Deutschland","koordinaten":{"lat":52.5,"lon":13.4}}}],"maxErgebnisse":1,"seite":1}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-API-Key") != apiKey {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(json))
	}))
	defer srv.Close()

	s := sut.NewWithURLs(srv.Client(), srv.URL, apiKey)
	jobs, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
	if jobs[0].CompanyName != "TechCorp" {
		t.Fatalf("expected company TechCorp, got %q", jobs[0].CompanyName)
	}
	if jobs[0].Location.City != "Berlin" {
		t.Fatalf("expected city Berlin, got %q", jobs[0].Location.City)
	}
}

func TestArbeitsagenturRequiresAPIKey(t *testing.T) {
	s := sut.New(nil)
	_, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 1})
	if err == nil {
		t.Fatal("expected error when API key is not set")
	}
	if !strings.Contains(err.Error(), "ARBEITSAGENTUR_API_KEY") {
		t.Fatalf("expected API key error, got: %v", err)
	}
}

func TestArbeitsagenturFailsOn401(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusUnauthorized) }))
	s := sut.NewWithURLs(srv.Client(), srv.URL, "bad-key")
	_, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 1})
	srv.Close()
	if err == nil {
		t.Fatal("expected error on 401")
	}
}

func TestArbeitsagenturReturnsErrorOn500(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusInternalServerError) }))
	s := sut.NewWithURLs(srv.Client(), srv.URL, "test-key")
	_, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 1})
	srv.Close()
	if err == nil {
		t.Fatal("expected error on 500")
	}
}
