package otta_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	sut "github.com/arinbalyan/scrappy/internal/scraper/otta"
)

func TestOttaFailsOnEmptyResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<!-- no jobs -->`))
	}))
	defer srv.Close()
	s := &sut.Scraper{Client: srv.Client(), ListURL: srv.URL + "/"}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := s.Scrape(ctx, model.ScraperInput{SearchTerm: "engineer", ResultsWanted: 5})
	if err == nil {
		t.Fatalf("expected error for empty upstream response, got nil")
	}
}

func TestOttaFailsOn429And503(t *testing.T) {
	for _, status := range []int{http.StatusTooManyRequests, http.StatusServiceUnavailable} {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(status)
		}))
		s := &sut.Scraper{Client: srv.Client(), ListURL: srv.URL + "/"}
		_, err := s.Scrape(ctx, model.ScraperInput{SearchTerm: "engineer", ResultsWanted: 5})
		srv.Close()
		if err == nil {
			t.Fatalf("expected error for status %d", status)
		}
	}
}

func TestOttaReturnsJobsOnValidHTML(t *testing.T) {
	page := `<!DOCTYPE html><html><body>
	<a href="https://otta.com/en/jobs/senior-go-engineer-abc123/">Senior Go Engineer</a>
	<a href="https://otta.com/en/jobs/backend-developer-def456/">Backend Developer</a>
	</body></html>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(page))
	}))
	defer srv.Close()
	s := &sut.Scraper{Client: srv.Client(), ListURL: srv.URL + "/"}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	jobs, err := s.Scrape(ctx, model.ScraperInput{SearchTerm: "engineer", ResultsWanted: 5})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(jobs) == 0 {
		t.Fatal("expected at least 1 job from valid HTML")
	}
	fmt.Printf("[otta test] scraped %d jobs\n", len(jobs))
}

func TestOttaParsesLDJSON(t *testing.T) {
	jobObj := map[string]any{
		"@type":       "JobPosting",
		"title":       "Go Engineer",
		"description": "Work on backend services",
		"datePosted":  "2025-12-01",
		"url":         "https://otta.com/en/jobs/go-eng-001",
		"hiringOrganization": map[string]string{"name": "TestCo"},
	}
	b, _ := json.Marshal(jobObj)
	ldScript := fmt.Sprintf(`<script type="application/ld+json">%s</script>`, string(b))
	page := `<!DOCTYPE html><html><body>` + ldScript + `</body></html>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(page))
	}))
	defer srv.Close()
	s := &sut.Scraper{Client: srv.Client(), ListURL: srv.URL + "/"}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	jobs, err := s.Scrape(ctx, model.ScraperInput{ResultsWanted: 5})
	if err != nil {
		t.Fatalf("unexpected error parsing LDJSON: %v", err)
	}
	if len(jobs) == 0 {
		t.Fatal("expected at least 1 job from LD+JSON")
	}
	if jobs[0].Title != "Go Engineer" {
		t.Fatalf("expected title 'Go Engineer', got %q", jobs[0].Title)
	}
}

func TestOttaDetectsChallengePage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<html><body><div>captcha required</div></body></html>`))
	}))
	defer srv.Close()
	s := &sut.Scraper{Client: srv.Client(), ListURL: srv.URL + "/"}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := s.Scrape(ctx, model.ScraperInput{SearchTerm: "engineer", ResultsWanted: 5})
	if err == nil {
		t.Fatal("expected error on challenge/captcha page")
	}
}
