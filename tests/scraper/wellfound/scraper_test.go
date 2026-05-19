package wellfound_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/arinbalyan/scrappy/internal/model"
	wellfoundpkg "github.com/arinbalyan/scrappy/internal/scraper/wellfound"
)

func TestWellfoundScrape(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<a href="https://wellfound.com/jobs/1"><h2>Go Engineer</h2><span class="company">Acme</span></a>`))
	}))
	defer srv.Close()
	s := wellfoundpkg.NewWithListURL(srv.Client(), srv.URL)
	jobs, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 1})
	if err != nil || len(jobs) != 1 || jobs[0].CompanyName != "Acme" { t.Fatalf("unexpected wellfound result: %v %+v", err, jobs) }
}
