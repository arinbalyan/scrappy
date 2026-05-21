package bayt_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	sut "github.com/arinbalyan/scrappy/internal/scraper/bayt"
)

func TestBaytFailsOnEmptyResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<!-- no jobs here -->`))
	}))
	defer srv.Close()
	s := &sut.Scraper{Client: srv.Client(), ListURL: srv.URL}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := s.Scrape(ctx, model.ScraperInput{SearchTerm: "engineer", ResultsWanted: 5})
	if err == nil {
		t.Fatalf("expected error for empty upstream response, got nil")
	}
}

func TestBaytFailsOn429And503(t *testing.T) {
	for _, status := range []int{http.StatusTooManyRequests, http.StatusServiceUnavailable} {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(status)
		}))
		s := &sut.Scraper{Client: srv.Client(), ListURL: srv.URL}
		_, err := s.Scrape(ctx, model.ScraperInput{SearchTerm: "engineer", ResultsWanted: 5})
		srv.Close()
		if err == nil {
			t.Fatalf("expected error for status %d", status)
		}
	}
}

func TestBaytReturnsJobsOnValidHTML(t *testing.T) {
	page := `<!DOCTYPE html><html><body>
	<a href="https://www.bayt.com/en/jobs/senior-go-engineer-abc123/"><strong>Senior Go Engineer</strong></a>
	<a href="https://www.bayt.com/en/jobs/backend-developer-def456/"><strong>Backend Developer</strong></a>
	</body></html>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(page))
	}))
	defer srv.Close()
	s := &sut.Scraper{Client: srv.Client(), ListURL: srv.URL}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	jobs, err := s.Scrape(ctx, model.ScraperInput{SearchTerm: "engineer", ResultsWanted: 5})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(jobs) == 0 {
		t.Fatal("expected at least 1 job from valid HTML")
	}
	fmt.Printf("[bayt test] scraped %d jobs\n", len(jobs))
}

func TestBaytDetectsChallengePage(t *testing.T) {
	challengePage := "Attention Required! | Cloudflare"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(challengePage))
	}))
	defer srv.Close()
	s := &sut.Scraper{Client: srv.Client(), ListURL: srv.URL}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := s.Scrape(ctx, model.ScraperInput{SearchTerm: "engineer", ResultsWanted: 5})
	if err == nil {
		t.Fatal("expected error on challenge/Cloudflare page")
	}
}
