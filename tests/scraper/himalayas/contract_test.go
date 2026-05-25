package himalayas_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	sut "github.com/arinbalyan/scrappy/internal/scraper/himalayas"
)

func TestHimalayasFailsOnEmptyResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"jobs":[],"totalCount":0,"offset":0,"limit":20}`))
	}))
	defer srv.Close()
	s := sut.NewWithAPIURL(srv.Client(), srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := s.Scrape(ctx, model.ScraperInput{SearchTerm: "software engineer", ResultsWanted: 5})
	if err == nil {
		t.Fatal("expected error for empty upstream response")
	}
}

func TestHimalayasFailsOn429And503(t *testing.T) {
	for _, status := range []int{http.StatusTooManyRequests, http.StatusServiceUnavailable} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(status)
			}))
			defer srv.Close()
			s := sut.NewWithAPIURL(srv.Client(), srv.URL)
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			_, err := s.Scrape(ctx, model.ScraperInput{SearchTerm: "software engineer", ResultsWanted: 5})
			if err == nil {
				t.Fatalf("expected error for status %d", status)
			}
		})
	}
}

func TestHimalayasScraper_Pagination(t *testing.T) {
	pageCalls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pageCalls++
		w.WriteHeader(http.StatusOK)
		if pageCalls == 1 {
			_, _ = w.Write([]byte(`{
				"jobs": [
					{
						"guid": "job1",
						"title": "First Page Job",
						"companyName": "Co A",
						"pubDate": 1717000000,
						"applicationLink": "https://himalayas.app/jobs/job1",
						"locationRestrictions": ["Remote"]
					}
				],
				"totalCount": 2,
				"offset": 0,
				"limit": 20
			}`))
		} else if pageCalls == 2 {
			_, _ = w.Write([]byte(`{
				"jobs": [
					{
						"guid": "job2",
						"title": "Second Page Job",
						"companyName": "Co B",
						"pubDate": 1717086400,
						"applicationLink": "https://himalayas.app/jobs/job2",
						"locationRestrictions": ["Remote"]
					}
				],
				"totalCount": 2,
				"offset": 1,
				"limit": 20
			}`))
		} else {
			_, _ = w.Write([]byte(`{"jobs":[],"totalCount":2,"offset":2,"limit":20}`))
		}
	}))
	defer srv.Close()

	s := sut.NewWithAPIURL(srv.Client(), srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	jobs, err := s.Scrape(ctx, model.ScraperInput{SearchTerm: "test", ResultsWanted: 10})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs from pagination, got %d", len(jobs))
	}
	if pageCalls < 2 {
		t.Fatalf("expected at least 2 page calls, got %d", pageCalls)
	}
	if jobs[0].ID != "himalayas-job1" {
		t.Fatalf("expected first job ID 'himalayas-job1', got %q", jobs[0].ID)
	}
	if jobs[1].ID != "himalayas-job2" {
		t.Fatalf("expected second job ID 'himalayas-job2', got %q", jobs[1].ID)
	}
}
