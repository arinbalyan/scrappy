package google_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/arinbalyan/scrappy/internal/model"
	googlepkg "github.com/arinbalyan/scrappy/internal/scraper/google"
)

func TestGoogleScrape(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<div data-job-id="abc"><div class="BjJfJf PUpOsf">Go Dev</div><div class="Qk80Jf">Acme</div></div>`))
	}))
	defer srv.Close()
	s := googlepkg.NewWithSearchURL(srv.Client(), srv.URL)
	jobs, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 1, SearchTerm: "go"})
	if err != nil || len(jobs) != 1 || jobs[0].CompanyName != "Acme" { t.Fatalf("unexpected google result: %v %+v", err, jobs) }
}
