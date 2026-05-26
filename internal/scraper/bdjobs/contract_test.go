package bdjobs

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/arinbalyan/scrappy/internal/model"
)

const testHTML = `<!DOCTYPE html>
<html>
<body>
<div class="norm-jobs-wrapper">
<div class="job-item">
<a href="jobdetail.asp?jobid=12345">Senior Software Engineer</a>
<div class="comp-name">Tech Corp Bangladesh</div>
<div class="locon">Dhaka, Bangladesh</div>
<div class="date">15 May 2026</div>
</div>
<div class="job-item">
<a href="jobdetail.asp?jobid=67890">Machine Learning Engineer</a>
<div class="comp-name">AI Solutions Ltd</div>
<div class="locon">Chittagong, Bangladesh</div>
<div class="date">16 May 2026</div>
</div>
<div class="job-item">
<a href="jobdetail.asp?jobid=11111">Remote DevOps Engineer</a>
<div class="comp-name">Cloud Inc BD</div>
<div class="locon">Remote</div>
<div class="date">17 May 2026</div>
</div>
</div>
</body>
</html>`

const emptyHTML = `<!DOCTYPE html>
<html><body><p>No jobs found</p></body></html>`

func TestScraper_SiteName(t *testing.T) {
	s := New(nil)
	if got := s.SiteName(); got != model.SiteBDJobs {
		t.Errorf("SiteName() = %q, want %q", got, model.SiteBDJobs)
	}
}

func TestScraper_Scrape(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(testHTML))
	}))
	defer ts.Close()

	s := NewWithSearchURL(nil, ts.URL)
	jobs, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 25})
	if err != nil {
		t.Fatalf("Scrape() returned error: %v", err)
	}

	if len(jobs) != 3 {
		t.Fatalf("expected 3 jobs, got %d", len(jobs))
	}

	// Check first job
	j0 := jobs[0]
	if j0.Title != "Senior Software Engineer" {
		t.Errorf("job[0].Title = %q, want %q", j0.Title, "Senior Software Engineer")
	}
	if j0.CompanyName != "Tech Corp Bangladesh" {
		t.Errorf("job[0].CompanyName = %q, want %q", j0.CompanyName, "Tech Corp Bangladesh")
	}
	if j0.Location.City != "Dhaka" {
		t.Errorf("job[0].Location.City = %q, want %q", j0.Location.City, "Dhaka")
	}
	if j0.Site != string(model.SiteBDJobs) {
		t.Errorf("job[0].Site = %q, want %q", j0.Site, model.SiteBDJobs)
	}
	expectedURL := "https://jobs.bdjobs.com/jobdetail.asp?jobid=12345"
	if j0.JobURL != expectedURL {
		t.Errorf("job[0].JobURL = %q, want %q", j0.JobURL, expectedURL)
	}
	if j0.IsRemote {
		t.Errorf("job[0].IsRemote should be false, got true")
	}

	// Check second job
	j1 := jobs[1]
	if j1.Title != "Machine Learning Engineer" {
		t.Errorf("job[1].Title = %q, want %q", j1.Title, "Machine Learning Engineer")
	}
	if j1.CompanyName != "AI Solutions Ltd" {
		t.Errorf("job[1].CompanyName = %q, want %q", j1.CompanyName, "AI Solutions Ltd")
	}

	// Check third job (remote)
	j2 := jobs[2]
	if j2.Title != "Remote DevOps Engineer" {
		t.Errorf("job[2].Title = %q, want %q", j2.Title, "Remote DevOps Engineer")
	}
	if !j2.IsRemote {
		t.Errorf("job[2].IsRemote should be true")
	}
}

func TestScraper_Scrape_EmptyBody(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(emptyHTML))
	}))
	defer ts.Close()

	s := NewWithSearchURL(nil, ts.URL)
	_, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 25})
	if err == nil {
		t.Fatal("expected error for empty results, got nil")
	}
}

func TestScraper_Scrape_HTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	s := NewWithSearchURL(nil, ts.URL)
	_, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 25})
	if err == nil {
		t.Fatal("expected error for 500 status, got nil")
	}
}
