package getonboard

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/arinbalyan/scrappy/internal/model"
)

func newTestJob(id string, title string, minSal, maxSal *float64, remote bool) getOnBoardJob {
	return getOnBoardJob{
		ID:   id,
		Type: "jobs",
		Attributes: getOnBoardAttributes{
			Title:       title,
			Description: "<p>Job description for " + title + "</p>",
			Company:     "TestCorp",
			Logo:        "https://logo.testcorp.com/logo.png",
			MinSalary:   minSal,
			MaxSalary:   maxSal,
			Remote:      remote,
			Seniority:   "senior",
			PublishedAt: int64Ptr(1715000000),
			Countries:   []string{"Chile"},
			LocationCities: []string{"Santiago"},
			Tags:        []string{"python", "go"},
		},
		Links: getOnBoardLinks{
			PublicURL: "https://www.getonbrd.com/jobs/" + id,
		},
	}
}

func int64Ptr(v int64) *int64 { return &v }

func float64Ptr(v float64) *float64 { return &v }

func TestScraper_SiteName(t *testing.T) {
	s := New(nil)
	if got := s.SiteName(); got != model.SiteGetOnBoard {
		t.Errorf("SiteName() = %q, want %q", got, model.SiteGetOnBoard)
	}
}

func TestScraper_Scrape(t *testing.T) {
	jobs := []getOnBoardJob{
		newTestJob("1", "Senior Go Developer", float64Ptr(60000), float64Ptr(85000), true),
		newTestJob("2", "Python Engineer", float64Ptr(45000), float64Ptr(70000), false),
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("query") == "" {
			// Job 3 has empty title and should be skipped
			job3 := newTestJob("3", "", nil, nil, false)
			resp := getOnBoardSearchResponse{Data: []getOnBoardJob{jobs[0], jobs[1], job3}}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
			return
		}
		// Filter by search term
		resp := getOnBoardSearchResponse{Data: []getOnBoardJob{jobs[1]}}
		w.Header().Set("Content-Type", "application/json")
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
	if j1.ID != "getonboard-1" {
		t.Errorf("job[0].ID = %q, want %q", j1.ID, "getonboard-1")
	}
	if j1.Title != "Senior Go Developer" {
		t.Errorf("job[0].Title = %q, want %q", j1.Title, "Senior Go Developer")
	}
	if j1.CompanyName != "TestCorp" {
		t.Errorf("job[0].CompanyName = %q, want %q", j1.CompanyName, "TestCorp")
	}
	if !j1.IsRemote {
		t.Error("job[0].IsRemote should be true")
	}
	if j1.Site != string(model.SiteGetOnBoard) {
		t.Errorf("job[0].Site = %q, want %q", j1.Site, model.SiteGetOnBoard)
	}
	if j1.Compensation == nil {
		t.Fatal("job[0].Compensation is nil")
	}
	if j1.Compensation.Currency != "USD" {
		t.Errorf("job[0].Compensation.Currency = %q, want %q", j1.Compensation.Currency, "USD")
	}
	if j1.Compensation.MinAmount == nil || math.Abs(*j1.Compensation.MinAmount-60000) > 0.01 {
		t.Errorf("job[0].Compensation.MinAmount = %v", *j1.Compensation.MinAmount)
	}
	if j1.DatePosted == nil {
		t.Error("job[0].DatePosted is nil")
	}

	// Check second job
	j2 := result[1]
	if j2.ID != "getonboard-2" {
		t.Errorf("job[1].ID = %q, want %q", j2.ID, "getonboard-2")
	}
	if j2.Title != "Python Engineer" {
		t.Errorf("job[1].Title = %q, want %q", j2.Title, "Python Engineer")
	}
	if j2.IsRemote {
		t.Error("job[1].IsRemote should be false")
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
		w.Write([]byte(`{"data":[]}`))
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
