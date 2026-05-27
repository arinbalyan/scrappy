package exa

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
	if got := s.SiteName(); got != model.SiteExa {
		t.Errorf("SiteName() = %q, want %q", got, model.SiteExa)
	}
}

func TestScraper_Scrape_MissingAPIKey(t *testing.T) {
	s := NewWithAPIKey(nil, "")
	_, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 25})
	if err == nil {
		t.Fatal("expected error when API key is missing, got nil")
	}
}

func TestScraper_Scrape_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != "test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		resp := exaResponse{
			Results: []exaResult{
				{
					URL:           "https://boards.greenhouse.io/acmecorp/jobs/123",
					Title:         "Software Engineer",
					Author:        "AcmeCorp",
					Text:          "We are looking for a Go developer.",
					Summary:       "Go developer position at AcmeCorp",
					PublishedDate: "2026-05-20T00:00:00Z",
				},
				{
					URL:           "https://remoteok.com/remote-devops-job",
					Title:         "DevOps Engineer",
					Text:          "Remote DevOps position.",
					PublishedDate: "2026-05-21T10:30:00Z",
				},
				{
					URL:   "https://example.com/",
					Title: "",
					Text:  "Some text but no title",
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	s := NewWithAPIKey(nil, "test-key")
	s.apiURL = ts.URL

	result, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 25})
	if err != nil {
		t.Fatalf("Scrape() returned error: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(result))
	}

	// Job 0
	j0 := result[0]
	if j0.Title != "Software Engineer" {
		t.Errorf("job[0].Title = %q, want %q", j0.Title, "Software Engineer")
	}
	if j0.CompanyName != "AcmeCorp" {
		t.Errorf("job[0].CompanyName = %q, want %q", j0.CompanyName, "AcmeCorp")
	}
	if j0.Site != string(model.SiteExa) {
		t.Errorf("job[0].Site = %q, want %q", j0.Site, model.SiteExa)
	}
	if j0.JobURL != "https://boards.greenhouse.io/acmecorp/jobs/123" {
		t.Errorf("job[0].JobURL = %q", j0.JobURL)
	}
	if !stringsContains(j0.ID, "exa-") {
		t.Errorf("job[0].ID should start with exa-, got %q", j0.ID)
	}
	if j0.DatePosted == nil {
		t.Error("job[0].DatePosted is nil")
	}

	// Job 1
	j1 := result[1]
	if j1.Title != "DevOps Engineer" {
		t.Errorf("job[1].Title = %q, want %q", j1.Title, "DevOps Engineer")
	}
	if j1.CompanyName != "" {
		t.Errorf("job[1].CompanyName = %q, want empty", j1.CompanyName)
	}
}

func TestScraper_Scrape_WithRemoteSearch(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		resp := exaResponse{
			Results: []exaResult{
				{
					URL:     "https://example.com/job/123",
					Title:   "Full Stack Developer",
					Text:    "Remote position - work from home.",
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	s := NewWithAPIKey(nil, "test-key")
	s.apiURL = ts.URL

	result, err := s.Scrape(context.Background(), model.ScraperInput{
		ResultsWanted: 25,
		IsRemote:      true,
		SearchTerm:    "developer",
	})
	if err != nil {
		t.Fatalf("Scrape() returned error: %v", err)
	}

	if len(result) != 1 {
		t.Fatalf("expected 1 job, got %d", len(result))
	}

	if !result[0].IsRemote {
		t.Error("job[0].IsRemote should be true (detected from description)")
	}
}

func TestScraper_Scrape_HTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	s := NewWithAPIKey(nil, "test-key")
	s.apiURL = ts.URL

	_, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 25})
	if err == nil {
		t.Fatal("expected error for 500 status, got nil")
	}
}

func TestScraper_Scrape_EmptyBody(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"results":[]}`))
	}))
	defer ts.Close()

	s := NewWithAPIKey(nil, "test-key")
	s.apiURL = ts.URL

	_, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 25})
	if err == nil {
		t.Fatal("expected error for empty results, got nil")
	}
}

func TestNewWithAPIURL_EmptyEndpoint(t *testing.T) {
	s1 := NewWithAPIURL(nil, "")
	s2 := New(nil)
	if s1.apiURL != s2.apiURL {
		t.Errorf("empty endpoint should not override API URL")
	}
}

func TestExtractCompanyFromURL(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"https://boards.greenhouse.io/acmecorp/jobs/123", "Acmecorp"},
		{"https://jobs.ashbyhq.com/mycompany", "Mycompany"},
		{"https://jobs.lever.co/startup-inc/xyz", "Startup Inc"},
		{"https://subdomain.lever.co/company-name", "Company Name"},
		{"https://apply.workable.com/techcorp/", "Techcorp"},
		{"https://linkedin.com/jobs/view/123", ""},
	}
	for _, tt := range tests {
		got := extractCompanyFromURL(tt.url)
		if got != tt.want {
			t.Errorf("extractCompanyFromURL(%q) = %q, want %q", tt.url, got, tt.want)
		}
	}
}

func TestHashURL(t *testing.T) {
	// Should be deterministic
	h1 := hashURL("https://example.com/job/123")
	h2 := hashURL("https://example.com/job/123")
	if h1 != h2 {
		t.Errorf("hashURL not deterministic: %q vs %q", h1, h2)
	}

	h3 := hashURL("https://example.com/job/456")
	if h1 == h3 {
		t.Errorf("hashURL collision: both %q", h1)
	}
}

func stringsContains(s, substr string) bool {
	return len(s) >= len(substr) && containsStr(s, substr)
}

func containsStr(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
