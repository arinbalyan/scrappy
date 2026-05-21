package androidjobs_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	sut "github.com/arinbalyan/scrappy/internal/scraper/androidjobs"
)

func TestAndroidjobsParsesRSS(t *testing.T) {
	rss := `<?xml version="1.0"?><rss><channel><item><title><![CDATA[Senior Android Engineer - Acme - Remote]]></title><link>https://androidjobs.io/jobs/senior-android-engineer</link><guid>senior-android-engineer</guid><description><![CDATA[<p>Build Android apps</p>]]></description><pubDate>Mon, 20 May 2026 12:00:00 +0000</pubDate></item></channel></rss>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(rss))
	}))
	defer srv.Close()

	s := sut.NewWithFeedURL(srv.Client(), srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	jobs, err := s.Scrape(ctx, model.ScraperInput{SearchTerm: "android", ResultsWanted: 1})
	if err != nil || len(jobs) != 1 {
		t.Fatalf("expected 1 job and nil error, got jobs=%d err=%v", len(jobs), err)
	}
}

func TestAndroidjobsFailsOn429And503(t *testing.T) {
	for _, status := range []int{http.StatusTooManyRequests, http.StatusServiceUnavailable} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(status) }))
		s := sut.NewWithFeedURL(srv.Client(), srv.URL)
		_, err := s.Scrape(context.Background(), model.ScraperInput{SearchTerm: "x", ResultsWanted: 1})
		srv.Close()
		if err == nil {
			t.Fatalf("expected error for status %d", status)
		}
	}
}
