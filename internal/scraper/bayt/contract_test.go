package bayt

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
<ul class="jobs-list">
<li data-js-job="1">
  <h2><a href="/en/international/job/senior-software-engineer-1001/">Senior Software Engineer</a></h2>
  <div class="t-mute t-small">Dubai, UAE</div>
  <div class="t-nowrap p10l"><span>Tech Corp</span></div>
</li>
<li data-js-job="2">
  <h2><a href="/en/international/job/machine-learning-engineer-1002/">Machine Learning Engineer</a></h2>
  <div class="t-mute t-small">Riyadh, Saudi Arabia</div>
  <div class="t-nowrap p10l"><span>AI Solutions</span></div>
</li>
<li data-js-job="3">
  <h2><a href="/en/international/job/devops-engineer-1003/">DevOps Engineer</a></h2>
  <div class="t-mute t-small">Cairo, Egypt</div>
  <div class="t-nowrap p10l"><span>Cloud Inc</span></div>
</li>
</ul>
</body>
</html>`

const emptyHTML = `<!DOCTYPE html>
<html><body><p>No jobs found</p></body></html>`

func TestScraper_SiteName(t *testing.T) {
	s := New(nil)
	if got := s.SiteName(); got != model.SiteBayt {
		t.Errorf("SiteName() = %q, want %q", got, model.SiteBayt)
	}
}

func TestScraper_Scrape(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(testHTML))
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

	// Check first job
	j0 := jobs[0]
	if j0.Title != "Senior Software Engineer" {
		t.Errorf("job[0].Title = %q, want %q", j0.Title, "Senior Software Engineer")
	}
	if j0.CompanyName != "Tech Corp" {
		t.Errorf("job[0].CompanyName = %q, want %q", j0.CompanyName, "Tech Corp")
	}
	if j0.Location.City != "Dubai, UAE" {
		t.Errorf("job[0].Location.City = %q, want %q", j0.Location.City, "Dubai, UAE")
	}
	if j0.Site != string(model.SiteBayt) {
		t.Errorf("job[0].Site = %q, want %q", j0.Site, model.SiteBayt)
	}
	expectedURL := ts.URL + "/en/international/job/senior-software-engineer-1001/"
	if j0.JobURL != expectedURL {
		t.Errorf("job[0].JobURL = %q, want %q", j0.JobURL, expectedURL)
	}

	// Check second job
	j1 := jobs[1]
	if j1.Title != "Machine Learning Engineer" {
		t.Errorf("job[1].Title = %q, want %q", j1.Title, "Machine Learning Engineer")
	}
	if j1.CompanyName != "AI Solutions" {
		t.Errorf("job[1].CompanyName = %q, want %q", j1.CompanyName, "AI Solutions")
	}

	// Check third job
	j2 := jobs[2]
	if j2.Title != "DevOps Engineer" {
		t.Errorf("job[2].Title = %q, want %q", j2.Title, "DevOps Engineer")
	}
}

func TestScraper_Scrape_EmptyBody(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(emptyHTML))
	}))
	defer ts.Close()

	s := NewWithBaseURL(nil, ts.URL)
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

	s := NewWithBaseURL(nil, ts.URL)
	_, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 25})
	if err == nil {
		t.Fatal("expected error for 500 status, got nil")
	}
}

func TestNewWithBaseURL_EmptyEndpoint(t *testing.T) {
	s1 := NewWithBaseURL(nil, "")
	s2 := New(nil)
	if s1.baseURL != s2.baseURL {
		t.Errorf("empty endpoint should not override base URL")
	}
}
