package freelancercom

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/arinbalyan/scrappy/internal/model"
)

func ptr(v int64) *int64         { return &v }
func fptr(v float64) *float64   { return &v }

func newProject(id int, title, projType string, minB, maxB *float64) freelancerProject {
	return freelancerProject{
		ID:            id,
		Title:         title,
		Description:   "Description for " + title,
		SeoURL:        fmt.Sprintf("test-project-%d", id),
		Type:          projType,
		Currency:      &freelancerCurrency{Code: "USD"},
		Budget:        &freelancerBudget{Minimum: minB, Maximum: maxB},
		TimeSubmitted: ptr(1715000000),
		Location: &freelancerLocation{
			City:    "San Francisco",
			Country: &freelancerCountry{Name: "USA"},
		},
		Owner: &freelancerOwner{
			Username:    "testuser",
			DisplayName: "TestCompany",
		},
	}
}

func TestScraper_SiteName(t *testing.T) {
	s := New(nil)
	if got := s.SiteName(); got != model.SiteFreelancerCom {
		t.Errorf("SiteName() = %q, want %q", got, model.SiteFreelancerCom)
	}
}

func TestScraper_Scrape(t *testing.T) {
	projects := []freelancerProject{
		newProject(1, "Build a Go API", "fixed", fptr(500), fptr(2000)),
		newProject(2, "Python Web Scraper", "hourly", nil, nil),
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := freelancerResponse{
			Status: "success",
			Result: &freelancerResult{Projects: projects, TotalCount: 2},
		}
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

	// Check first job (fixed price project)
	j1 := result[0]
	if j1.ID != "freelancercom-1" {
		t.Errorf("job[0].ID = %q, want %q", j1.ID, "freelancercom-1")
	}
	if j1.Title != "Build a Go API" {
		t.Errorf("job[0].Title = %q, want %q", j1.Title, "Build a Go API")
	}
	if j1.CompanyName != "TestCompany" {
		t.Errorf("job[0].CompanyName = %q, want %q", j1.CompanyName, "TestCompany")
	}
	if !j1.IsRemote {
		t.Error("job[0].IsRemote should be true")
	}
	if j1.Site != string(model.SiteFreelancerCom) {
		t.Errorf("job[0].Site = %q, want %q", j1.Site, model.SiteFreelancerCom)
	}
	if j1.Compensation == nil {
		t.Fatal("job[0].Compensation is nil")
	}
	if j1.Compensation.Interval != model.IntervalYearly {
		t.Errorf("job[0].Compensation.Interval = %q, want %q", j1.Compensation.Interval, model.IntervalYearly)
	}
	if j1.Compensation.MinAmount == nil || math.Abs(*j1.Compensation.MinAmount-500) > 0.01 {
		t.Errorf("job[0].Compensation.MinAmount = %v", j1.Compensation.MinAmount)
	}
	if j1.DatePosted == nil {
		t.Error("job[0].DatePosted is nil")
	}
	if j1.Location.City != "San Francisco" {
		t.Errorf("job[0].Location.City = %q", j1.Location.City)
	}

	// Check second job (hourly project, nil budget)
	j2 := result[1]
	if j2.ID != "freelancercom-2" {
		t.Errorf("job[1].ID = %q, want %q", j2.ID, "freelancercom-2")
	}
	if j2.Title != "Python Web Scraper" {
		t.Errorf("job[1].Title = %q, want %q", j2.Title, "Python Web Scraper")
	}
	if j2.Compensation != nil {
		t.Error("job[1].Compensation should be nil (no budget)")
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
		w.Write([]byte(`{"status":"success","result":{"projects":[],"total_count":0}}`))
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
