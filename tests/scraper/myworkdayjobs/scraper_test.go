package myworkdayjobs_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/arinbalyan/scrappy/internal/model"
	workday "github.com/arinbalyan/scrappy/internal/scraper/myworkdayjobs"
)

func TestWorkdayScrape_FilterExperienceRemote(t *testing.T) {
	t.Setenv("SCRAPPY_WORKDAY_SEEDS", "")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"jobPostings":[{"title":"Senior Software Engineer","externalPath":"job/123","postedOn":"2026-05-19","locationsText":"Remote, US","jobDescription":"Need 5+ years Golang","remoteType":"REMOTE"},{"title":"HR Generalist","externalPath":"job/999","postedOn":"2026-05-10","locationsText":"Austin, US","jobDescription":"People ops"}]}`))
	}))
	defer srv.Close()

	s := workday.New(srv.Client())
	jobs, err := s.Scrape(context.Background(), model.ScraperInput{SearchTerm: "software engineer", WorkdaySeeds: []string{srv.URL + "/wday/cxs/acme/external/jobs"}})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 got %d", len(jobs))
	}
	if jobs[0].ExperienceRange == "" || !jobs[0].IsRemote {
		t.Fatalf("expected experience and remote true, got %+v", jobs[0])
	}
}
