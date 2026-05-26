package wellfound

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/arinbalyan/scrappy/internal/model"
)

const testPageHTML = `<!DOCTYPE html>
<html>
<body>
<script id="__NEXT_DATA__" type="application/json">
{
  "props": {
    "pageProps": {
      "listings": [
        {
          "id": "job001",
          "title": "Senior Go Developer",
          "slug": "senior-go-developer-at-techcorp",
          "company": {
            "name": "TechCorp",
            "slug": "techcorp",
            "logoUrl": "https://logo.example.com/techcorp.png"
          },
          "compensation": {
            "min": 150000,
            "max": 200000,
            "currency": "USD"
          },
          "locations": ["San Francisco, CA"],
          "remote": true,
          "description": "<p>Build amazing Go services.</p>",
          "skills": ["Go", "PostgreSQL", "Kubernetes"],
          "createdAt": "2026-05-20T10:00:00Z"
        },
        {
          "id": "job002",
          "title": "Frontend Engineer",
          "slug": "frontend-engineer-at-webinc",
          "company": {
            "name": "WebInc",
            "slug": "webinc"
          },
          "compensation": {
            "max": 180000,
            "currency": "USD"
          },
          "locations": ["Remote"],
          "remote": true,
          "description": "React and TypeScript.",
          "skills": ["React", "TypeScript"],
          "createdAt": "2026-05-19T08:30:00Z"
        },
        {
          "id": "job003",
          "title": "Designer",
          "slug": "",
          "company": null,
          "locations": [],
          "remote": false,
          "description": "",
          "skills": [],
          "createdAt": ""
        }
      ]
    }
  }
}
</script>
</body>
</html>`

func TestScraper_SiteName(t *testing.T) {
	s := New(nil)
	if got := s.SiteName(); got != model.SiteWellfound {
		t.Errorf("SiteName() = %q, want %q", got, model.SiteWellfound)
	}
}

func TestScraper_Scrape(t *testing.T) {
	pageNum := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pageNum++
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		if pageNum == 1 {
			w.Write([]byte(testPageHTML))
		} else {
			w.Write([]byte("<html><body></body></html>"))
		}
	}))
	defer ts.Close()

	s := NewWithBaseURL(nil, ts.URL)
	jobs, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 25})
	if err != nil {
		t.Fatalf("Scrape() returned error: %v", err)
	}

	if len(jobs) != 3 {
		t.Fatalf("expected 3 jobs, got %d", len(jobs))
	}

	// Job 0: Senior Go Developer
	j0 := jobs[0]
	if j0.Title != "Senior Go Developer" {
		t.Errorf("job[0].Title = %q, want %q", j0.Title, "Senior Go Developer")
	}
	if j0.CompanyName != "TechCorp" {
		t.Errorf("job[0].CompanyName = %q, want %q", j0.CompanyName, "TechCorp")
	}
	if j0.Site != string(model.SiteWellfound) {
		t.Errorf("job[0].Site = %q, want %q", j0.Site, model.SiteWellfound)
	}
	if !strings.Contains(j0.JobURL, "senior-go-developer-at-techcorp") {
		t.Errorf("job[0].JobURL = %q, should contain slug", j0.JobURL)
	}
	if j0.Compensation == nil {
		t.Fatal("job[0].Compensation is nil")
	}
	if j0.Compensation.Interval != "yearly" {
		t.Errorf("job[0].Compensation.Interval = %q, want yearly", j0.Compensation.Interval)
	}
	if j0.Compensation.MinAmount == nil || *j0.Compensation.MinAmount != 150000 {
		t.Errorf("job[0].Compensation.MinAmount = %v, want 150000", j0.Compensation.MinAmount)
	}
	if !j0.IsRemote {
		t.Error("job[0].IsRemote = false, want true")
	}
	if j0.Location.City != "San Francisco, CA" {
		t.Errorf("job[0].Location.City = %q", j0.Location.City)
	}
	if len(j0.Skills) != 3 {
		t.Errorf("job[0].Skills = %v, want 3 skills", j0.Skills)
	}
	if j0.DatePosted == nil {
		t.Error("job[0].DatePosted is nil")
	}
	if j0.CompanyLogoURL != "https://logo.example.com/techcorp.png" {
		t.Errorf("job[0].CompanyLogoURL = %q", j0.CompanyLogoURL)
	}

	// Job 1: Frontend Engineer
	j1 := jobs[1]
	if j1.Title != "Frontend Engineer" {
		t.Errorf("job[1].Title = %q", j1.Title)
	}
	if j1.Compensation == nil || j1.Compensation.MinAmount != nil {
		t.Errorf("job[1].Compensation.MinAmount should be nil (only max set)")
	}
	if j1.Compensation.MaxAmount == nil || *j1.Compensation.MaxAmount != 180000 {
		t.Errorf("job[1].Compensation.MaxAmount = %v", j1.Compensation.MaxAmount)
	}

	// Job 2: Designer (no company)
	j2 := jobs[2]
	if j2.Title != "Designer" {
		t.Errorf("job[2].Title = %q", j2.Title)
	}
	if j2.CompanyName != "" {
		t.Errorf("job[2].CompanyName = %q, want empty", j2.CompanyName)
	}
	if j2.Compensation != nil {
		t.Error("job[2].Compensation should be nil")
	}
}

func TestScraper_Scrape_HTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	s := NewWithBaseURL(nil, ts.URL)
	_, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 25})
	if err == nil {
		t.Fatal("expected error for 500 status, got nil")
	}
}

func TestScraper_Scrape_NoNextData(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("<html><body>No data here</body></html>"))
	}))
	defer ts.Close()

	s := NewWithBaseURL(nil, ts.URL)
	_, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 25})
	if err == nil {
		t.Fatal("expected error for missing __NEXT_DATA__, got nil")
	}
}

func TestScraper_Scrape_EmptyResponse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`<html><body><script id="__NEXT_DATA__" type="application/json">{"props":{"pageProps":{"listings":[]}}}</script></body></html>`))
	}))
	defer ts.Close()

	s := NewWithBaseURL(nil, ts.URL)
	_, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 25})
	if err == nil {
		t.Fatal("expected error for empty listings, got nil")
	}
}

func TestScraper_Scrape_429(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer ts.Close()

	s := NewWithBaseURL(nil, ts.URL)
	_, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 25})
	if err == nil {
		t.Fatal("expected error for 429 status, got nil")
	}
}

func TestExtractListings(t *testing.T) {
	// Test with valid data
	listings, err := extractListings([]byte(testPageHTML))
	if err != nil {
		t.Fatalf("extractListings() returned error: %v", err)
	}
	if len(listings) != 3 {
		t.Fatalf("expected 3 listings, got %d", len(listings))
	}

	// Test with no __NEXT_DATA__
	_, err = extractListings([]byte("<html></html>"))
	if err == nil {
		t.Error("expected error when __NEXT_DATA__ not found")
	}
}

func TestMapListing(t *testing.T) {
	l := listing{
		ID:    "123",
		Title: "Test Role",
		Slug:  "test-role",
		Company: &company{
			Name: "TestCo",
			Slug: "testco",
		},
		Comp: &compField{
			Min:      float64Ptr(100000),
			Max:      float64Ptr(150000),
			Currency: "USD",
		},
		Locations: []string{"Remote"},
		Remote:    true,
		Desc:      "<p>A test role.</p>",
		Skills:    []string{"Go"},
		CreatedAt: "2026-05-20T10:00:00Z",
	}

	job := mapListing(l)
	if job.Title != "Test Role" {
		t.Errorf("Title = %q", job.Title)
	}
	if job.CompanyName != "TestCo" {
		t.Errorf("CompanyName = %q", job.CompanyName)
	}
	if job.Location.City != "Remote" {
		t.Errorf("Location.City = %q", job.Location.City)
	}
	if !job.IsRemote {
		t.Error("IsRemote should be true")
	}
	if len(job.Skills) != 1 || job.Skills[0] != "Go" {
		t.Errorf("Skills = %v", job.Skills)
	}
	if job.ID != "wellfound-123" {
		t.Errorf("ID = %q", job.ID)
	}
}

func float64Ptr(f float64) *float64 { return &f }
