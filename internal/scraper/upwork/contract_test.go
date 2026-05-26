package upwork

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/arinbalyan/scrappy/internal/model"
)

const testGraphQLResponse = `{
  "data": {
    "marketplaceJobPostings": {
      "totalCount": 3,
      "edges": [
        {
          "node": {
            "id": "job001",
            "ciphertext": "~01abc123",
            "title": "Senior Go Developer",
            "description": "Looking for an experienced Go developer.",
            "createdDateTime": "2026-05-20T10:00:00Z",
            "duration": "3-6 months",
            "engagement": "Full-time",
            "amount": {
              "amount": "5000",
              "currencyCode": "USD"
            },
            "category": {"name": "Web Development"},
            "subcategory": {"name": "Backend Development"},
            "skills": [{"name": "Go"}, {"name": "PostgreSQL"}],
            "client": {"totalPostedJobs": 10, "totalHires": 5},
            "contractorTier": "EXPERT"
          }
        },
        {
          "node": {
            "id": "job002",
            "ciphertext": "",
            "title": "Remote Data Analyst",
            "description": "Analyze data from remote. Work from home ok.",
            "createdDateTime": "2026-05-19T08:30:00Z",
            "duration": "",
            "engagement": "Part-time",
            "weeklyBudget": {
              "amount": "1000",
              "currencyCode": "USD"
            },
            "category": {"name": "Data Science"},
            "subcategory": {"name": "Data Analysis"},
            "skills": [{"name": "Python"}, {"name": "SQL"}],
            "client": {"totalPostedJobs": 25, "totalHires": 15},
            "contractorTier": "INTERMEDIATE"
          }
        },
        {
          "node": {
            "id": "",
            "ciphertext": "",
            "title": "",
            "description": "",
            "createdDateTime": "",
            "client": null
          }
        }
      ]
    }
  }
}`

func TestScraper_SiteName(t *testing.T) {
	s := NewWithClient(nil, "test-id", "test-secret")
	if got := s.SiteName(); got != model.SiteUpwork {
		t.Errorf("SiteName() = %q, want %q", got, model.SiteUpwork)
	}
}

func TestScraper_IsConfigured(t *testing.T) {
	s1 := New(nil)
	if s1.IsConfigured() {
		t.Error("New() without env vars should report not configured")
	}

	s2 := NewWithClient(nil, "id", "secret")
	if !s2.IsConfigured() {
		t.Error("NewWithClient() should report configured")
	}
}

func TestScraper_Scrape(t *testing.T) {
	tokenCalled := false
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "token") {
			tokenCalled = true
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{
				"access_token": "test-token-123",
				"token_type":   "bearer",
				"expires_in":   3600,
				"scope":        "hr_skills_cw_jobs_search",
			})
		} else if strings.Contains(r.URL.Path, "graphql") {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(testGraphQLResponse))
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	s := NewWithBaseURL(nil, ts.URL, "test-client-id", "test-client-secret")
	jobs, err := s.Scrape(context.Background(), model.ScraperInput{
		ResultsWanted: 25,
		SearchTerm:    "Go developer",
	})
	if err != nil {
		t.Fatalf("Scrape() returned error: %v", err)
	}

	if !tokenCalled {
		t.Error("token endpoint was not called")
	}

	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(jobs))
	}

	// Job 0: Senior Go Developer
	j0 := jobs[0]
	if j0.Title != "Senior Go Developer" {
		t.Errorf("job[0].Title = %q, want %q", j0.Title, "Senior Go Developer")
	}
	if j0.CompanyName != "Upwork Client" {
		t.Errorf("job[0].CompanyName = %q, want %q", j0.CompanyName, "Upwork Client")
	}
	if j0.Site != string(model.SiteUpwork) {
		t.Errorf("job[0].Site = %q, want %q", j0.Site, model.SiteUpwork)
	}
	if !strings.Contains(j0.JobURL, "~01abc123") {
		t.Errorf("job[0].JobURL = %q, should contain ciphertext", j0.JobURL)
	}
	if j0.Compensation == nil {
		t.Fatal("job[0].Compensation is nil")
	}
	if j0.Compensation.Currency != "USD" {
		t.Errorf("job[0].Compensation.Currency = %q, want USD", j0.Compensation.Currency)
	}
	if j0.DatePosted == nil {
		t.Error("job[0].DatePosted is nil")
	}
	if !strings.Contains(j0.Description, "Skills: Go, PostgreSQL") {
		t.Errorf("job[0].Description missing skills: %s", j0.Description)
	}
	if j0.JobType != "fulltime" {
		t.Errorf("job[0].JobType = %q, want fulltime", j0.JobType)
	}

	// Job 1: Remote Data Analyst
	j1 := jobs[1]
	if j1.Title != "Remote Data Analyst" {
		t.Errorf("job[1].Title = %q, want %q", j1.Title, "Remote Data Analyst")
	}
	if !j1.IsRemote {
		t.Error("job[1].IsRemote should be true")
	}
	if j1.JobType != "parttime" {
		t.Errorf("job[1].JobType = %q, want parttime", j1.JobType)
	}
}

func TestScraper_Scrape_NoCredentials(t *testing.T) {
	s := New(nil)
	_, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 10})
	if err == nil {
		t.Fatal("expected error when no credentials configured, got nil")
	}
}

func TestScraper_Scrape_HTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	s := NewWithBaseURL(nil, ts.URL, "id", "secret")
	_, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 10})
	if err == nil {
		t.Fatal("expected error for 500 status, got nil")
	}
}

func TestProcessNode(t *testing.T) {
	// Test full node
	node := jobPostingNode{
		ID:              "123",
		Ciphertext:      "~abc",
		Title:           "Test Job",
		Description:     "A test job for testing",
		CreatedDateTime: "2026-05-20T10:00:00Z",
		Engagement:      "Full-time",
		Amount:          &amountField{Amount: "5000", CurrencyCode: "USD"},
		Category:        &namedField{Name: "Engineering"},
		Subcategory:     &namedField{Name: "Software"},
		Skills:          []skillField{{Name: "Go"}, {Name: "Python"}},
		Client:          &clientField{TotalPostedJobs: intPtr(10), TotalHires: intPtr(5)},
		ContractorTier:  "EXPERT",
	}

	job, err := processNode(node)
	if err != nil {
		t.Fatalf("processNode() returned error: %v", err)
	}

	if job.ID != "upwork-123" {
		t.Errorf("job.ID = %q", job.ID)
	}
	if job.Title != "Test Job" {
		t.Errorf("job.Title = %q", job.Title)
	}
	if job.Compensation == nil {
		t.Fatal("compensation is nil")
	}
	if *job.Compensation.MinAmount != 5000 {
		t.Errorf("MinAmount = %f, want 5000", *job.Compensation.MinAmount)
	}

	// Test empty node
	_, err = processNode(jobPostingNode{})
	if err == nil {
		t.Error("expected error for empty node")
	}

	// Test weekly budget
	node2 := jobPostingNode{
		ID:            "456",
		Title:         "Test 2",
		WeeklyBudget:  &amountField{Amount: "1000", CurrencyCode: "USD"},
	}
	job2, err := processNode(node2)
	if err != nil {
		t.Fatalf("processNode() returned error: %v", err)
	}
	if job2.Compensation.Interval != "weekly" {
		t.Errorf("Interval = %q, want weekly", job2.Compensation.Interval)
	}
}

func TestHumanizeTier(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"ENTRY", "Entry Level"},
		{"INTERMEDIATE", "Intermediate"},
		{"EXPERT", "Expert"},
		{"UNKNOWN", "UNKNOWN"},
	}
	for _, tt := range tests {
		got := humanizeTier(tt.input)
		if got != tt.want {
			t.Errorf("humanizeTier(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func intPtr(i int) *int { return &i }
