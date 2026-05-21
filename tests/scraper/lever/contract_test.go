package lever_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	sut "github.com/arinbalyan/scrappy/internal/scraper/lever"
)

func TestLeverFailsOnEmptyResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()
	s := sut.New(srv.Client())
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := s.Scrape(ctx, model.ScraperInput{SearchTerm: "test-co", ResultsWanted: 5})
	if err == nil {
		t.Fatalf("expected error for empty postings response, got nil")
	}
}

func TestLeverFailsOn429And503(t *testing.T) {
	for _, status := range []int{http.StatusTooManyRequests, http.StatusServiceUnavailable} {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(status)
		}))
		s := sut.New(srv.Client())
		_, err := s.Scrape(ctx, model.ScraperInput{SearchTerm: "test-co", ResultsWanted: 5})
		srv.Close()
		if err == nil {
			t.Fatalf("expected error for status %d", status)
		}
	}
}

func TestLeverReturnsErrorOn403(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	s := sut.New(srv.Client())
	_, err := s.Scrape(ctx, model.ScraperInput{SearchTerm: "test-co", ResultsWanted: 5})
	srv.Close()
	if err == nil {
		t.Fatal("expected error on 403")
	}
}

func TestLeverReturnsErrorWhenNoSeeds(t *testing.T) {
	s := sut.New(nil)
	_, err := s.Scrape(context.Background(), model.ScraperInput{})
	if err == nil {
		t.Fatal("expected error when no seeds are available")
	}
}
