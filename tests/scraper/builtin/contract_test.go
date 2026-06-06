package builtin_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	sut "github.com/arinbalyan/scrappy/internal/scraper/builtin"
)

func TestBuiltinParsesNextData(t *testing.T) {
	html := `<script id="__NEXT_DATA__">{"props":{"pageProps":{"jobs":[{"id":1,"title":"Staff Engineer","url":"/job/staff-engineer","company_name":"Acme","city_name":"San Francisco","state_name":"CA","country_name":"US","salary_min":180000,"salary_max":220000,"remote_type":"Remote","created":"2026-01-01T00:00:00Z"}]}}}</script>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(html))
	}))
	defer srv.Close()
	s := sut.NewWithBaseURL(srv.Client(), srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	jobs, err := s.Scrape(ctx, model.ScraperInput{SearchTerm: "engineer", ResultsWanted: 1})
	if err != nil || len(jobs) != 1 {
		t.Fatalf("expected 1 job and nil error, got jobs=%d err=%v", len(jobs), err)
	}
}

func TestBuiltinParsesDataIDHTML(t *testing.T) {
	html := `<!DOCTYPE html><html><body>
	<div class="job-list">
	<div data-id="job-card" class="job-bounded-responsive position-relative bg-white p-md rounded-3">
	<div id="main" class="row">
	<div class="col-12 col-lg-7 left-side-tile">
	<div class="left-side-tile-item-1"><a href="/company/acme" target="_blank"><picture><img data-id="company-img" src="logo.png" alt="Acme Logo"/></picture></a></div>
	<div class="left-side-tile-item-2"><a href="/company/acme" target="_blank" data-id="company-title" class="fw-medium"><span>Acme Corp</span></a></div>
	<div class="left-side-tile-item-3"><h2><a href="/job/staff-engineer/12345" target="_blank" data-id="job-card-title" class="text-break">Staff Engineer</a></h2></div>
	</div>
	<div class="col-12 col-lg-5">
	<div><span class="font-barlow text-gray-04">Remote</span></div>
	<div><i class="fa-regular fa-location-dot"></i><div><span class="font-barlow text-gray-04">San Francisco, CA, USA</span></div></div>
	<div><i class="fa-regular fa-sack-dollar"></i><span class="font-barlow text-gray-04">155K-170K Annually</span></div>
	</div>
	</div>
	</div>
	</div></body></html>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(html))
	}))
	defer srv.Close()
	s := sut.NewWithBaseURL(srv.Client(), srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	jobs, err := s.Scrape(ctx, model.ScraperInput{SearchTerm: "", ResultsWanted: 5})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
	if jobs[0].Title != "Staff Engineer" {
		t.Fatalf("expected title 'Staff Engineer', got %q", jobs[0].Title)
	}
	if jobs[0].CompanyName != "Acme Corp" {
		t.Fatalf("expected company 'Acme Corp', got %q", jobs[0].CompanyName)
	}
	if jobs[0].JobURL != "https://builtin.com/job/staff-engineer/12345" {
		t.Fatalf("expected URL 'https://builtin.com/job/staff-engineer/12345', got %q", jobs[0].JobURL)
	}
	if jobs[0].Location.City != "San Francisco, CA, USA" {
		t.Fatalf("expected location 'San Francisco, CA, USA', got %q", jobs[0].Location.City)
	}
	if jobs[0].Compensation == nil || jobs[0].Compensation.MinAmount == nil || *jobs[0].Compensation.MinAmount != 155000 {
		t.Fatalf("expected min salary 155000, got %v", jobs[0].Compensation)
	}
}

func TestBuiltinFailsOn429And503(t *testing.T) {
	for _, status := range []int{http.StatusTooManyRequests, http.StatusServiceUnavailable} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(status) }))
		s := sut.NewWithBaseURL(srv.Client(), srv.URL)
		_, err := s.Scrape(context.Background(), model.ScraperInput{SearchTerm: "x", ResultsWanted: 1})
		srv.Close()
		if err == nil {
			t.Fatalf("expected error for status %d", status)
		}
	}
}
