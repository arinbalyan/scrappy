package tesla

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/arinbalyan/scrappy/internal/model"
)

const testBoardResponse = `{
  "listings": [
    {"id": "job001", "t": "Software Engineer", "l": "loc_palo_alto", "d": "dept_eng"},
    {"id": "job002", "t": "Product Manager", "l": "loc_remote", "d": "dept_product"},
    {"id": "job003", "t": "Designer"}
  ],
  "lookup": {
    "locations": {
      "loc_palo_alto": "Palo Alto, CA",
      "loc_remote": "Remote"
    },
    "departments": {
      "dept_eng": "Engineering",
      "dept_product": "Product"
    }
  }
}`

const testDetailResponse = `{
  "jobDescription": "Build awesome things.",
  "jobResponsibilities": "Code, test, ship.",
  "jobRequirements": "5+ years Go experience.",
  "jobCompensationAndBenefits": "Competitive salary + equity."
}`

func TestScraper_SiteName(t *testing.T) {
	s := New(nil)
	if got := s.SiteName(); got != model.SiteTesla {
		t.Errorf("SiteName() = %q, want %q", got, model.SiteTesla)
	}
}

func TestScraper_Scrape(t *testing.T) {
	detailCallCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "careers/state") {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(testBoardResponse))
		} else if strings.Contains(r.URL.Path, "careers/job") {
			detailCallCount++
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(testDetailResponse))
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	s := NewWithBaseURL(nil, ts.URL)
	jobs, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 10})
	if err != nil {
		t.Fatalf("Scrape() returned error: %v", err)
	}

	if len(jobs) != 3 {
		t.Fatalf("expected 3 jobs, got %d", len(jobs))
	}

	// Job 0: Software Engineer
	j0 := jobs[0]
	if j0.Title != "Software Engineer" {
		t.Errorf("job[0].Title = %q, want %q", j0.Title, "Software Engineer")
	}
	if j0.CompanyName != "Tesla" {
		t.Errorf("job[0].CompanyName = %q, want %q", j0.CompanyName, "Tesla")
	}
	if j0.Site != string(model.SiteTesla) {
		t.Errorf("job[0].Site = %q, want %q", j0.Site, model.SiteTesla)
	}
	if j0.Location.City != "Palo Alto, CA" {
		t.Errorf("job[0].Location.City = %q, want %q", j0.Location.City, "Palo Alto, CA")
	}
	if j0.IsRemote {
		t.Error("job[0].IsRemote = true, want false")
	}
	if j0.Department != "Engineering" {
		t.Errorf("job[0].Department = %q, want %q", j0.Department, "Engineering")
	}
	if j0.Description == "" {
		t.Error("job[0].Description is empty, expected detail fetch")
	}
	if !strings.Contains(j0.Description, "Build awesome things") {
		t.Errorf("job[0].Description missing 'Build awesome things'")
	}
	if !strings.Contains(j0.JobURL, "software-engineer-job001") {
		t.Errorf("job[0].JobURL = %q, should contain software-engineer-job001", j0.JobURL)
	}

	// Job 1: Product Manager (remote)
	j1 := jobs[1]
	if j1.Title != "Product Manager" {
		t.Errorf("job[1].Title = %q, want %q", j1.Title, "Product Manager")
	}
	if !j1.IsRemote {
		t.Error("job[1].IsRemote = false, want true")
	}
	if j1.Department != "Product" {
		t.Errorf("job[1].Department = %q, want %q", j1.Department, "Product")
	}

	// Job 2: Designer (no location/department keys)
	j2 := jobs[2]
	if j2.Title != "Designer" {
		t.Errorf("job[2].Title = %q, want %q", j2.Title, "Designer")
	}
	if j2.Location.City != "" {
		t.Errorf("job[2].Location.City = %q, want empty", j2.Location.City)
	}
	if j2.Department != "" {
		t.Errorf("job[2].Department = %q, want empty", j2.Department)
	}

	// Should have fetched details for all 3 jobs
	if detailCallCount != 3 {
		t.Errorf("expected 3 detail fetches, got %d", detailCallCount)
	}
}

func TestScraper_Scrape_HTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	s := NewWithBaseURL(nil, ts.URL)
	_, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 10})
	if err == nil {
		t.Fatal("expected error for 500 status, got nil")
	}
}

func TestScraper_Scrape_EmptyResponse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"listings":[],"lookup":{}}`))
	}))
	defer ts.Close()

	s := NewWithBaseURL(nil, ts.URL)
	_, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 10})
	if err == nil {
		t.Fatal("expected error for empty response, got nil")
	}
}

func TestScraper_Scrape_AkamaiChallenge(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("<html><body>Please enable JavaScript</body></html>"))
	}))
	defer ts.Close()

	s := NewWithBaseURL(nil, ts.URL)
	_, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 10})
	if err == nil {
		t.Fatal("expected error for Akamai challenge, got nil")
	}
}

func TestScraper_Scrape_429(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer ts.Close()

	s := NewWithBaseURL(nil, ts.URL)
	_, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 10})
	if err == nil {
		t.Fatal("expected error for 429 status, got nil")
	}
}

func TestNewWithBaseURL_EmptyEndpoint(t *testing.T) {
	s1 := NewWithBaseURL(nil, "")
	s2 := New(nil)
	if s1.baseURL != s2.baseURL {
		t.Errorf("empty endpoint should not override base URL")
	}
}

func TestBuildJobURL(t *testing.T) {
	tests := []struct {
		id    string
		title string
		want  string
	}{
		{"job001", "Software Engineer", "https://www.tesla.com/careers/search/job/software-engineer-job001"},
		{"abc", "Senior Staff Engineer II", "https://www.tesla.com/careers/search/job/senior-staff-engineer-ii-abc"},
		{"xyz", "C++ Developer", "https://www.tesla.com/careers/search/job/c-developer-xyz"},
	}
	for _, tt := range tests {
		got := buildJobURL(tt.id, tt.title)
		if got != tt.want {
			t.Errorf("buildJobURL(%q, %q) = %q, want %q", tt.id, tt.title, got, tt.want)
		}
	}
}

func TestComposeDescription(t *testing.T) {
	// Test with all fields
	d := &jobDetail{
		JobDescription:             strPtr("Desc"),
		JobResponsibilities:        strPtr("Resp"),
		JobRequirements:            strPtr("Req"),
		JobCompensationAndBenefits: strPtr("Comp"),
	}
	desc := composeDescription(d)
	if desc == "" {
		t.Fatal("composeDescription returned empty for non-nil detail")
	}
	if !strings.Contains(desc, "Description:") || !strings.Contains(desc, "Responsibilities:") {
		t.Error("composeDescription missing section headers")
	}

	// Test with nil fields
	d2 := &jobDetail{}
	if composeDescription(d2) != "" {
		t.Error("composeDescription should return empty for empty detail")
	}

	// Test nil
	if composeDescription(nil) != "" {
		t.Error("composeDescription should return empty for nil")
	}
}

func strPtr(s string) *string { return &s }
