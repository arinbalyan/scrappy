package hackernews_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	sut "github.com/arinbalyan/scrappy/internal/scraper/hackernews"
)

func TestHackernewsParsesJobs(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/stories", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[101,102]`))
	})
	mux.HandleFunc("/item/101", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":101,"title":"Acme is hiring Go Engineer","url":"https://acme.com/jobs/101","text":"<p>Remote role</p>","time":1716200000}`))
	})
	mux.HandleFunc("/item/102", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":102,"title":"Other role","text":"<p>No match</p>","time":1716200001}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	s := sut.NewWithURLs(srv.Client(), srv.URL+"/stories", srv.URL+"/item/%d")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	jobs, err := s.Scrape(ctx, model.ScraperInput{SearchTerm: "engineer", ResultsWanted: 1})
	if err != nil || len(jobs) != 1 {
		t.Fatalf("expected 1 job and nil error, got jobs=%d err=%v", len(jobs), err)
	}
	if jobs[0].CompanyName == "" {
		t.Fatalf("expected company name extraction")
	}
}

func TestHackernewsFailsOnStoriesError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusTooManyRequests) }))
	defer srv.Close()
	s := sut.NewWithURLs(srv.Client(), srv.URL, srv.URL+"/%d")
	_, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 1})
	if err == nil || !strings.Contains(err.Error(), "status") {
		t.Fatalf("expected status error, got %v", err)
	}
	_ = fmt.Sprintf("%v", err)
}
