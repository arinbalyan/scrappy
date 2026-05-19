package remotive_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/arinbalyan/scrappy/internal/model"
	remotivepkg "github.com/arinbalyan/scrappy/internal/scraper/remotive"
)

func TestRemotiveScrape(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"jobs":[{"id":1,"title":"Go Dev","company_name":"Acme","candidate_required_location":"Remote","publication_date":"2026-05-20T00:00:00Z","url":"https://example.com/job","salary":"","description":"desc","category":"Software"}]}`))
	}))
	defer srv.Close()
	s := remotivepkg.NewWithAPIURL(srv.Client(), srv.URL)
	jobs, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 1})
	if err != nil || len(jobs) != 1 || jobs[0].Title != "Go Dev" { t.Fatalf("unexpected remotive result: %v %+v", err, jobs) }
}
