package ziprecruiter_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/arinbalyan/scrappy/internal/model"
	ziprecruiterpkg "github.com/arinbalyan/scrappy/internal/scraper/ziprecruiter"
)

func TestZipRecruiterScrape(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<div data-job-id="z1"><a class="job_content" href="https://ziprecruiter.com/jobs/z1"><h2>Senior Engineer</h2></a><a class="t_org_link">Acme</a></div>`))
	}))
	defer srv.Close()
	s := ziprecruiterpkg.NewWithListURL(srv.Client(), srv.URL)
	jobs, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 1})
	if err != nil || len(jobs) != 1 || jobs[0].CompanyName != "Acme" { t.Fatalf("unexpected ziprecruiter result: %v %+v", err, jobs) }
}

func TestZipRecruiterScrapePrefersLDJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<script type="application/ld+json">{"@type":"JobPosting","title":"Principal Engineer","description":"Platform engineering","datePosted":"2026-05-20T12:00:00Z","url":"https://ziprecruiter.com/jobs/z42","hiringOrganization":{"name":"Acme"}}</script>`))
	}))
	defer srv.Close()
	s := ziprecruiterpkg.NewWithListURL(srv.Client(), srv.URL)
	jobs, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 1})
	if err != nil || len(jobs) != 1 || jobs[0].Title != "Principal Engineer" {
		t.Fatalf("unexpected ziprecruiter ldjson result: %v %+v", err, jobs)
	}
	if jobs[0].DatePosted == nil || jobs[0].CompanyName != "Acme" {
		t.Fatalf("expected parsed ldjson fields: %+v", jobs[0])
	}
}
