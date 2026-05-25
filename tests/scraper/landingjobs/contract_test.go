package landingjobs_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	sut "github.com/arinbalyan/scrappy/internal/scraper/landingjobs"
)

func TestLandingJobsFailsOnEmptyResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	s := sut.NewWithAPIURL(srv.Client(), srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := s.Scrape(ctx, model.ScraperInput{SearchTerm: "engineer", ResultsWanted: 5})
	if err == nil {
		t.Fatal("expected error for empty upstream response")
	}
}

func TestLandingJobsFailsOn429And503(t *testing.T) {
	for _, status := range []int{http.StatusTooManyRequests, http.StatusServiceUnavailable} {
		t.Run(fmt.Sprintf("status_%d", status), func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(status)
			}))
			defer srv.Close()

			s := sut.NewWithAPIURL(srv.Client(), srv.URL)
			_, err := s.Scrape(context.Background(), model.ScraperInput{
				SearchTerm:    "engineer",
				ResultsWanted: 1,
			})
			if err == nil {
				t.Fatalf("expected error for status %d", status)
			}
			if !strings.Contains(err.Error(), fmt.Sprintf("%d", status)) {
				t.Errorf("expected error to contain status %d, got %v", status, err)
			}
		})
	}
}

func TestLandingJobsScraper_ScrapeBasic(t *testing.T) {
	pageCalls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pageCalls++
		if pageCalls == 1 {
			// Verify pagination params
			if r.URL.Query().Get("offset") != "0" {
				t.Errorf("expected offset=0, got %q", r.URL.Query().Get("offset"))
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[
				{
					"id": 101,
					"title": "Software Engineer",
					"city": "Lisbon",
					"country_name": "Portugal",
					"currency_code": "EUR",
					"salary_low": 50000,
					"salary_high": 80000,
					"remote": true,
					"role_description": "Build great software",
					"tags": ["Engineering", "Backend"],
					"published_at": "2026-05-20T10:00:00Z"
				},
				{
					"id": 102,
					"title": "Product Manager",
					"city": "Berlin",
					"country_name": "Germany",
					"remote": false,
					"role_description": "Lead product strategy",
					"tags": ["Product"],
					"published_at": "2026-05-19T08:00:00Z"
				}
			]`))
		} else {
			// Second page returns empty to stop pagination
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[]`))
		}
	}))
	defer srv.Close()

	s := sut.NewWithAPIURL(srv.Client(), srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	jobs, err := s.Scrape(ctx, model.ScraperInput{
		SearchTerm:    "",
		ResultsWanted: 10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(jobs))
	}

	// --- First job: full fields ---
	j1 := jobs[0]
	if j1.ID != "landingjobs-101" {
		t.Fatalf("expected ID 'landingjobs-101', got %q", j1.ID)
	}
	if j1.Title != "Software Engineer" {
		t.Fatalf("expected title 'Software Engineer', got %q", j1.Title)
	}
	if j1.JobURL != "https://landing.jobs/jobs/101" {
		t.Fatalf("expected job URL, got %q", j1.JobURL)
	}
	if j1.Location.City != "Lisbon" {
		t.Fatalf("expected city 'Lisbon', got %q", j1.Location.City)
	}
	if j1.Location.Country != "Portugal" {
		t.Fatalf("expected country 'Portugal', got %q", j1.Location.Country)
	}
	if !j1.IsRemote {
		t.Fatal("expected IsRemote = true")
	}
	if j1.Description != "Build great software" {
		t.Fatalf("expected description, got %q", j1.Description)
	}
	if j1.Compensation == nil {
		t.Fatal("expected compensation")
	} else {
		if j1.Compensation.Interval != model.IntervalYearly {
			t.Fatalf("expected yearly interval, got %s", j1.Compensation.Interval)
		}
		if j1.Compensation.MinAmount == nil || *j1.Compensation.MinAmount != 50000 {
			t.Fatalf("expected min 50000, got %v", j1.Compensation.MinAmount)
		}
		if j1.Compensation.Currency != "EUR" {
			t.Fatalf("expected currency EUR, got %s", j1.Compensation.Currency)
		}
	}
	if j1.DatePosted == nil {
		t.Fatal("expected date_posted to be set")
	}
	if len(j1.Skills) != 2 || j1.Skills[0] != "Engineering" {
		t.Fatalf("expected skills [Engineering, Backend], got %v", j1.Skills)
	}

	// --- Second job: no remote, no compensation ---
	j2 := jobs[1]
	if j2.ID != "landingjobs-102" {
		t.Fatalf("expected ID 'landingjobs-102', got %q", j2.ID)
	}
	if j2.Title != "Product Manager" {
		t.Fatalf("expected title 'Product Manager', got %q", j2.Title)
	}
	if j2.IsRemote {
		t.Fatal("expected IsRemote = false")
	}
	if j2.Compensation != nil {
		t.Fatal("expected no compensation")
	}
	if j2.Location.City != "Berlin" {
		t.Fatalf("expected city 'Berlin', got %q", j2.Location.City)
	}
}

func TestLandingJobsScraper_SearchFilter(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusOK)
		if calls == 1 {
			_, _ = w.Write([]byte(`[
				{
					"id": 201,
					"title": "Senior Golang Developer",
					"remote": true,
					"role_description": "Build Go backends",
					"tags": ["Engineering", "Go"],
					"published_at": "2026-05-20T10:00:00Z"
				},
				{
					"id": 202,
					"title": "Marketing Manager",
					"remote": false,
					"role_description": "Lead marketing team",
					"tags": ["Marketing"],
					"published_at": "2026-05-19T08:00:00Z"
				}
			]`))
		} else {
			_, _ = w.Write([]byte(`[]`))
		}
	}))
	defer srv.Close()

	s := sut.NewWithAPIURL(srv.Client(), srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Search for "golang" — should only match the first job
	jobs, err := s.Scrape(ctx, model.ScraperInput{
		SearchTerm:    "golang",
		ResultsWanted: 10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job (search filter 'golang'), got %d", len(jobs))
	}
	if jobs[0].ID != "landingjobs-201" {
		t.Fatalf("expected job 201, got %s", jobs[0].ID)
	}
}

func TestLandingJobsScraper_Pagination(t *testing.T) {
	pageCalls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pageCalls++
		w.WriteHeader(http.StatusOK)
		if pageCalls == 1 {
			_, _ = w.Write([]byte(`[
				{
					"id": 301,
					"title": "Page 1 Job",
					"remote": true,
					"published_at": "2026-05-20T10:00:00Z"
				}
			]`))
		} else if pageCalls == 2 {
			_, _ = w.Write([]byte(`[
				{
					"id": 302,
					"title": "Page 2 Job",
					"remote": true,
					"published_at": "2026-05-21T10:00:00Z"
				}
			]`))
		} else {
			_, _ = w.Write([]byte(`[]`))
		}
	}))
	defer srv.Close()

	s := sut.NewWithAPIURL(srv.Client(), srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	jobs, err := s.Scrape(ctx, model.ScraperInput{
		SearchTerm:    "",
		ResultsWanted: 10,
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs from pagination, got %d", len(jobs))
	}
	if pageCalls < 2 {
		t.Fatalf("expected at least 2 page calls, got %d", pageCalls)
	}
	if jobs[0].ID != "landingjobs-301" {
		t.Fatalf("expected first job ID 'landingjobs-301', got %q", jobs[0].ID)
	}
	if jobs[1].ID != "landingjobs-302" {
		t.Fatalf("expected second job ID 'landingjobs-302', got %q", jobs[1].ID)
	}
}
