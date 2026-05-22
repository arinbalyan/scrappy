package devitjobs_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	sut "github.com/arinbalyan/scrappy/internal/scraper/devitjobs"
)

func TestDevITJobsParsesXML(t *testing.T) {
	xml := `<?xml version="1.0"?><jobs><job><title><![CDATA[Senior Go Developer]]></title><link>https://devitjobs.com/jobs/999</link><description><![CDATA[<p>Distributed systems</p>]]></description><company><![CDATA[Acme]]></company><location><![CDATA[Remote]]></location><salary><![CDATA[CHF 120'000 - 140'000]]></salary><pubDate>Mon, 20 May 2026 12:00:00 +0000</pubDate><category>Engineering</category><type>Remote</type></job></jobs>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(xml))
	}))
	defer srv.Close()

	s := sut.NewWithFeedURL(srv.Client(), srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	jobs, err := s.Scrape(ctx, model.ScraperInput{SearchTerm: "developer", ResultsWanted: 1})
	if err != nil || len(jobs) != 1 {
		t.Fatalf("expected 1 job and nil error, got jobs=%d err=%v", len(jobs), err)
	}
}

func TestDevITJobsFailsOn429And503(t *testing.T) {
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
