package ziprecruiter_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	sut "github.com/arinbalyan/scrappy/internal/scraper/ziprecruiter"
)

func TestZipRecruiterParsesJobsAndContinueToken(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/jobs-app/event") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ok":true}`))
			return
		}
		calls++
		if calls == 1 {
			if got := r.URL.Query().Get("search"); got != "software engineer" {
				t.Fatalf("expected search query, got %q", got)
			}
			if got := r.URL.Query().Get("location"); got != "Remote" {
				t.Fatalf("expected location query, got %q", got)
			}
			_, _ = w.Write([]byte(`{"jobs":[{"job_id":"123","name":"Backend Engineer","job_url":"https://example.com/job/123","apply_url":"https://example.com/apply/123","job_description":"Build APIs","employment_type":"full_time","remote":"true","job_city":"San Francisco","job_state":"CA","job_country":"US","salary_min_annual":120000,"salary_max_annual":160000,"hiring_company":{"name":"Acme","url":"https://acme.com","logo":"https://acme.com/logo.png"}}],"continue_token":"next-1"}`))
			return
		}
		if calls == 2 {
			if got := r.URL.Query().Get("continue_token"); got != "next-1" {
				t.Fatalf("expected continue token next-1, got %q", got)
			}
			_, _ = w.Write([]byte(`{"jobs":[{"id":"124","title":"Platform Engineer","url":"https://example.com/job/124","snippet":"Scale systems","employment_type":"contractor","remote":"false","hiring_company":{"name":"Beta"}}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"jobs":[]}`))
	}))
	defer srv.Close()

	s := sut.NewWithURLs(srv.Client(), srv.URL+"/jobs-app/jobs", srv.URL+"/jobs-app/event")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	jobs, err := s.Scrape(ctx, model.ScraperInput{SearchTerm: "software engineer", Location: "Remote", ResultsWanted: 2})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(jobs))
	}
	if jobs[0].ID != "zr-123" || jobs[0].Title != "Backend Engineer" || jobs[0].JobType != "fulltime" || !jobs[0].IsRemote {
		t.Fatalf("unexpected first job: %+v", jobs[0])
	}
	if jobs[0].Compensation == nil || jobs[0].Compensation.MinAmount == nil || *jobs[0].Compensation.MinAmount != 120000 {
		t.Fatalf("expected yearly compensation on first job")
	}
	if jobs[1].ID != "zr-124" || jobs[1].Title != "Platform Engineer" {
		t.Fatalf("unexpected second job: %+v", jobs[1])
	}
}

func TestZipRecruiterFailsOnEmptyResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/jobs-app/event") {
			w.WriteHeader(http.StatusOK)
			return
		}
		_, _ = w.Write([]byte(`{"jobs":[]}`))
	}))
	defer srv.Close()
	s := sut.NewWithURLs(srv.Client(), srv.URL+"/jobs-app/jobs", srv.URL+"/jobs-app/event")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := s.Scrape(ctx, model.ScraperInput{SearchTerm: "software engineer", ResultsWanted: 5})
	if err == nil {
		t.Fatalf("expected error on empty upstream response")
	}
}

func TestZipRecruiterFailsOn429And503(t *testing.T) {
	for _, status := range []int{http.StatusTooManyRequests, http.StatusServiceUnavailable} {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.Contains(r.URL.Path, "/jobs-app/event") {
				w.WriteHeader(http.StatusOK)
				return
			}
			w.WriteHeader(status)
		}))
		s := sut.NewWithURLs(srv.Client(), srv.URL+"/jobs-app/jobs", srv.URL+"/jobs-app/event")
		_, err := s.Scrape(ctx, model.ScraperInput{SearchTerm: "software engineer", ResultsWanted: 5})
		srv.Close()
		if err == nil {
			t.Fatalf("expected error for status %d", status)
		}
	}
}
