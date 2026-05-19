package linkedin_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/arinbalyan/scrappy/internal/model"
	linkedinpkg "github.com/arinbalyan/scrappy/internal/scraper/linkedin"
)

func TestLinkedInScraper_ScrapeBasic(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/jobs-guest/jobs/api/seeMoreJobPostings/search" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`
<div class="base-search-card">
  <a class="base-card__full-link" href="/jobs/view/1234567890?tracking">link</a>
  <span class="sr-only">Senior Go Engineer</span>
  <h4 class="base-search-card__subtitle"><a href="https://www.linkedin.com/company/acme">Acme</a></h4>
  <span class="job-search-card__location">San Francisco, CA</span>
  <time datetime="2026-05-01"></time>
  <span class="job-search-card__salary-info">$120,000 - $150,000</span>
</div>`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<html></html>"))
	}))
	defer server.Close()

	s := linkedinpkg.NewWithBaseURL(server.Client(), server.URL)
	jobs, err := s.Scrape(context.Background(), model.ScraperInput{SearchTerm: "golang", ResultsWanted: 1})
	if err != nil {
		t.Fatalf("scrape: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
	if jobs[0].Title != "Senior Go Engineer" {
		t.Fatalf("unexpected title: %q", jobs[0].Title)
	}
	if jobs[0].Compensation == nil || jobs[0].Compensation.MinAmount == nil {
		t.Fatalf("expected compensation parsed")
	}
}
