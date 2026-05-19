package remoteok_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/arinbalyan/scrappy/internal/model"
	remoteokpkg "github.com/arinbalyan/scrappy/internal/scraper/remoteok"
)

func TestRemoteOKScrape(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"legal":"ok"},{"id":9,"position":"Backend Engineer","company":"Acme","url":"https://remoteok.com/l/backend","epoch":1716160000}]`))
	}))
	defer srv.Close()
	s := remoteokpkg.NewWithAPIURL(srv.Client(), srv.URL)
	jobs, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 1})
	if err != nil || len(jobs) != 1 || jobs[0].Title != "Backend Engineer" { t.Fatalf("unexpected remoteok result: %v %+v", err, jobs) }
}
