package joinrise

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
)

const testJSON = `{
  "result": {
    "jobs": [
      {
        "_id": "abc123",
        "title": "Senior Software Engineer",
        "url": "https://joinrise.ai/jobs/senior-software-engineer",
        "locationAddress": "San Francisco, CA",
        "type": "Remote",
        "createdAt": "2026-05-15T10:00:00Z",
        "owner": {
          "companyName": "TechCorp",
          "photo": "https://company.com/logo.png"
        },
        "descriptionBreakdown": {
          "oneSentenceJobSummary": "Build scalable backend systems.",
          "salaryRangeMinYearly": 150000,
          "salaryRangeMaxYearly": 200000,
          "keywords": ["Go", "Kubernetes", "AWS"],
          "workModel": "Remote"
        }
      },
      {
        "_id": "def456",
        "title": "Data Scientist",
        "url": "https://joinrise.ai/jobs/data-scientist",
        "locationAddress": "New York, NY",
        "type": "On-site",
        "createdAt": "2026-05-16T14:30:00Z",
        "owner": {
          "companyName": "AI Labs",
          "photo": ""
        },
        "descriptionBreakdown": {
          "oneSentenceJobSummary": "Design and implement ML models.",
          "salaryRangeMinYearly": null,
          "salaryRangeMaxYearly": null,
          "keywords": ["Python", "ML"],
          "workModel": "On-site"
        }
      },
      {
        "_id": "ghi789",
        "title": "",
        "url": "https://joinrise.ai/jobs/empty",
        "owner": {
          "companyName": "Empty Inc",
          "photo": ""
        }
      }
    ],
    "count": 3
  }
}`

func TestScraper_SiteName(t *testing.T) {
	s := New(nil)
	if got := s.SiteName(); got != model.SiteJoinRise {
		t.Errorf("SiteName() = %q, want %q", got, model.SiteJoinRise)
	}
}

func TestScraper_Scrape(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("limit") != "25" {
			t.Errorf("expected limit=25, got %q", r.URL.Query().Get("limit"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(testJSON))
	}))
	defer ts.Close()

	s := NewWithAPIURL(nil, ts.URL)
	jobs, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 25})
	if err != nil {
		t.Fatalf("Scrape() returned error: %v", err)
	}

	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(jobs))
	}

	// Check first job (remote, with salary)
	j0 := jobs[0]
	if j0.ID != "joinrise-abc123" {
		t.Errorf("job[0].ID = %q, want %q", j0.ID, "joinrise-abc123")
	}
	if j0.Title != "Senior Software Engineer" {
		t.Errorf("job[0].Title = %q", j0.Title)
	}
	if j0.CompanyName != "TechCorp" {
		t.Errorf("job[0].CompanyName = %q", j0.CompanyName)
	}
	if j0.CompanyLogo != "https://company.com/logo.png" {
		t.Errorf("job[0].CompanyLogo = %q", j0.CompanyLogo)
	}
	if j0.JobURL != "https://joinrise.ai/jobs/senior-software-engineer" {
		t.Errorf("job[0].JobURL = %q", j0.JobURL)
	}
	if j0.Location.City != "San Francisco, CA" {
		t.Errorf("job[0].Location.City = %q", j0.Location.City)
	}
	if !j0.IsRemote {
		t.Error("job[0].IsRemote should be true")
	}
	if j0.Site != string(model.SiteJoinRise) {
		t.Errorf("job[0].Site = %q", j0.Site)
	}
	if j0.DatePosted == nil {
		t.Error("job[0].DatePosted is nil")
	} else if !j0.DatePosted.Before(time.Now()) {
		t.Error("job[0].DatePosted should be in the past")
	}
	if j0.Compensation == nil {
		t.Fatal("job[0].Compensation is nil")
	}
	if j0.Compensation.Interval != model.IntervalYearly {
		t.Errorf("job[0].Compensation.Interval = %q", j0.Compensation.Interval)
	}
	if j0.Compensation.Currency != "USD" {
		t.Errorf("job[0].Compensation.Currency = %q", j0.Compensation.Currency)
	}
	if j0.Compensation.MinAmount == nil || *j0.Compensation.MinAmount != 150000 {
		t.Errorf("job[0].Compensation.MinAmount = %v", j0.Compensation.MinAmount)
	}

	// Check second job (on-site, no salary)
	j1 := jobs[1]
	if j1.ID != "joinrise-def456" {
		t.Errorf("job[1].ID = %q", j1.ID)
	}
	if j1.Title != "Data Scientist" {
		t.Errorf("job[1].Title = %q", j1.Title)
	}
	if j1.CompanyName != "AI Labs" {
		t.Errorf("job[1].CompanyName = %q", j1.CompanyName)
	}
	if j1.IsRemote {
		t.Error("job[1].IsRemote should be false")
	}
	if j1.Compensation != nil {
		t.Error("job[1].Compensation should be nil")
	}
}

func TestScraper_Scrape_SearchTerm(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(testJSON))
	}))
	defer ts.Close()

	s := NewWithAPIURL(nil, ts.URL)
	jobs, err := s.Scrape(context.Background(), model.ScraperInput{
		SearchTerm:    "Data Scientist",
		ResultsWanted: 25,
	})
	if err != nil {
		t.Fatalf("Scrape() returned error: %v", err)
	}

	if len(jobs) != 1 {
		t.Fatalf("expected 1 job matching 'Data Scientist', got %d", len(jobs))
	}
	if jobs[0].Title != "Data Scientist" {
		t.Errorf("job[0].Title = %q", jobs[0].Title)
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
		w.Write([]byte(`{"result":{"jobs":[],"count":0}}`))
	}))
	defer ts.Close()

	s := NewWithAPIURL(nil, ts.URL)
	_, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 25})
	if err == nil {
		t.Fatal("expected error for empty jobs, got nil")
	}
}

func TestNewWithAPIURL_EmptyEndpoint(t *testing.T) {
	s1 := NewWithAPIURL(nil, "")
	s2 := New(nil)
	if s1.apiURL != s2.apiURL {
		t.Errorf("empty endpoint should not override API URL")
	}
}
