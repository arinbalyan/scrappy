package himalayas_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/arinbalyan/scrappy/internal/model"
	himalayas "github.com/arinbalyan/scrappy/internal/scraper/himalayas"
)

func TestHimalayasScrape(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		// Verify pagination params on first call
		if callCount == 1 {
			if r.URL.Query().Get("limit") != "5" {
				t.Errorf("expected limit=5, got %q", r.URL.Query().Get("limit"))
			}
			if r.URL.Query().Get("offset") != "0" {
				t.Errorf("expected offset=0, got %q", r.URL.Query().Get("offset"))
			}
			_, _ = w.Write([]byte(`{
				"jobs": [
					{
						"guid": "abc123",
						"title": "Senior AI Engineer",
						"companyName": "Acme AI",
						"companyLogo": "https://logo.example.com/acme.png",
						"employmentType": "Full Time",
						"minSalary": 100000,
						"maxSalary": 200000,
						"seniority": ["Senior"],
						"currency": "USD",
						"locationRestrictions": ["Worldwide"],
						"description": "Build LLM systems",
						"pubDate": 1717000000,
						"applicationLink": "https://himalayas.app/jobs/abc123"
					},
					{
						"guid": "abc124",
						"title": "AI Platform Engineer",
						"companyName": "Acme AI",
						"employmentType": "Contract",
						"locationRestrictions": ["Remote"],
						"description": "Ship infra",
						"pubDate": 1717086400,
						"applicationLink": "https://himalayas.app/jobs/abc124"
					}
				],
				"totalCount": 2,
				"offset": 0,
				"limit": 20
			}`))
		} else {
			// Return empty to prevent further paging
			_, _ = w.Write([]byte(`{"jobs":[],"totalCount":2,"offset":5,"limit":20}`))
		}
	}))
	defer srv.Close()

	s := himalayas.NewWithAPIURL(srv.Client(), srv.URL)
	jobs, err := s.Scrape(context.Background(), model.ScraperInput{SearchTerm: "ai", ResultsWanted: 5})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(jobs))
	}

	// First job: check all fields
	j0 := jobs[0]
	if j0.ID != "himalayas-abc123" {
		t.Fatalf("expected ID 'himalayas-abc123', got %q", j0.ID)
	}
	if j0.Title != "Senior AI Engineer" {
		t.Fatalf("expected title 'Senior AI Engineer', got %q", j0.Title)
	}
	if j0.CompanyName != "Acme AI" {
		t.Fatalf("expected company 'Acme AI', got %q", j0.CompanyName)
	}
	if j0.CompanyLogoURL != "https://logo.example.com/acme.png" {
		t.Fatalf("expected logo, got %q", j0.CompanyLogoURL)
	}
	if !j0.IsRemote {
		t.Fatal("expected IsRemote = true")
	}
	if j0.JobType != "full time" {
		t.Fatalf("expected job type 'full time', got %q", j0.JobType)
	}
	if j0.Description != "Build LLM systems" {
		t.Fatalf("expected description, got %q", j0.Description)
	}
	if j0.Location.City != "Worldwide" {
		t.Fatalf("expected location 'Worldwide', got %q", j0.Location.City)
	}
	if j0.Compensation == nil {
		t.Fatal("expected compensation")
	} else {
		if j0.Compensation.Interval != model.IntervalYearly {
			t.Fatalf("expected yearly interval, got %s", j0.Compensation.Interval)
		}
		if j0.Compensation.MinAmount == nil || *j0.Compensation.MinAmount != 100000 {
			t.Fatalf("expected min 100000, got %v", j0.Compensation.MinAmount)
		}
		if j0.Compensation.MaxAmount == nil || *j0.Compensation.MaxAmount != 200000 {
			t.Fatalf("expected max 200000, got %v", j0.Compensation.MaxAmount)
		}
		if j0.Compensation.Currency != "USD" {
			t.Fatalf("expected currency USD, got %s", j0.Compensation.Currency)
		}
	}
	if j0.Seniority != "Senior" {
		t.Fatalf("expected seniority 'Senior', got %q", j0.Seniority)
	}
	if j0.JobLevel != "Senior" {
		t.Fatalf("expected job level 'Senior', got %q", j0.JobLevel)
	}
	if j0.DatePosted == nil {
		t.Fatal("expected date_posted")
	}

	// Second job: no compensation, default values
	j1 := jobs[1]
	if j1.ID != "himalayas-abc124" {
		t.Fatalf("expected ID 'himalayas-abc124', got %q", j1.ID)
	}
	if j1.Title != "AI Platform Engineer" {
		t.Fatalf("expected title 'AI Platform Engineer', got %q", j1.Title)
	}
	if j1.Location.City != "Remote" {
		t.Fatalf("expected location 'Remote', got %q", j1.Location.City)
	}
	if j1.Compensation != nil {
		t.Fatal("expected no compensation for second job")
	}
	if j1.JobType != "contract" {
		t.Fatalf("expected job type 'contract', got %q", j1.JobType)
	}
}
