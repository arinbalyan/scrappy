package glassdoor_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/arinbalyan/scrappy/internal/model"
	glassdoorpkg "github.com/arinbalyan/scrappy/internal/scraper/glassdoor"
)

func TestGlassdoorScrape(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<div data-jobid="g1"><a class="jobLink">Engineer</a><span class="EmployerProfile_compactEmployerName">Acme</span></div>`))
	}))
	defer srv.Close()
	s := glassdoorpkg.NewWithListURL(srv.Client(), srv.URL)
	jobs, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 1})
	if err != nil || len(jobs) != 1 || jobs[0].Title != "Engineer" { t.Fatalf("unexpected glassdoor result: %v %+v", err, jobs) }
}
