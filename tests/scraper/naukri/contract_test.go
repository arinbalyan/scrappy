package naukri_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	sut "github.com/arinbalyan/scrappy/internal/scraper/naukri"
)

func TestNaukriParsesAPI(t *testing.T) {
	jsonBody := `{"jobDetails":[{"jobId":"abc123","title":"Senior Go Engineer","companyName":"Acme","jdURL":"/job-listings-go-abc123","jobDescription":"Build systems","createdDate":1710000000000,"placeholders":[{"type":"location","label":"Bengaluru, Karnataka"},{"type":"salary","label":"10 - 20 Lacs"}],"tagsAndSkills":"Go,Microservices"}]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(jsonBody))
	}))
	defer srv.Close()
	s := sut.NewWithAPIURL(srv.Client(), srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	jobs, err := s.Scrape(ctx, model.ScraperInput{SearchTerm: "golang", ResultsWanted: 1})
	if err != nil || len(jobs) != 1 {
		t.Fatalf("expected 1 job and nil error, got jobs=%d err=%v", len(jobs), err)
	}
	if jobs[0].Compensation == nil || jobs[0].Compensation.Currency != "INR" {
		t.Fatalf("expected INR compensation")
	}
}

func TestNaukriFailsOn429And503(t *testing.T) {
	for _, status := range []int{http.StatusTooManyRequests, http.StatusServiceUnavailable} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(status) }))
		s := sut.NewWithAPIURL(srv.Client(), srv.URL)
		_, err := s.Scrape(context.Background(), model.ScraperInput{SearchTerm: "x", ResultsWanted: 1})
		srv.Close()
		if err == nil {
			t.Fatalf("expected error for status %d", status)
		}
	}
}
