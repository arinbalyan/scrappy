package academiccareers_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	sut "github.com/arinbalyan/scrappy/internal/scraper/academiccareers"
)

func TestAcademicCareersParsesJobs(t *testing.T) {
	rss := `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:dc="http://purl.org/dc/elements/1.1/">
<channel>
  <title>AcademicCareers</title>
  <link>https://www.academiccareers.com</link>
  <item>
    <title>Assistant Professor of Computer Science</title>
    <link>https://www.academiccareers.com/job/12345</link>
    <guid>ac-12345</guid>
    <description>Tenure-track position in CS. Requirements: PhD and publications.</description>
    <pubDate>Mon, 15 Mar 2025 00:00:00 GMT</pubDate>
    <dc:creator>Stanford University</dc:creator>
  </item>
  <item>
    <title>Postdoc in Machine Learning</title>
    <link>https://www.academiccareers.com/job/67890</link>
    <guid>ac-67890</guid>
    <description>2-year postdoc in ML at MIT.</description>
    <pubDate>Tue, 16 Mar 2025 00:00:00 GMT</pubDate>
    <dc:creator>MIT</dc:creator>
  </item>
</channel>
</rss>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(rss))
	}))
	defer srv.Close()

	s := sut.NewWithURLs(srv.Client(), srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	jobs, err := s.Scrape(ctx, model.ScraperInput{ResultsWanted: 10})
	if err != nil || len(jobs) != 2 {
		t.Fatalf("expected 2 jobs and nil error, got jobs=%d err=%v", len(jobs), err)
	}
	if jobs[0].CompanyName != "Stanford University" {
		t.Fatalf("expected company Stanford University, got %q", jobs[0].CompanyName)
	}
	if jobs[1].CompanyName != "MIT" {
		t.Fatalf("expected company MIT, got %q", jobs[1].CompanyName)
	}
}

func TestAcademicCareersFiltersBySearch(t *testing.T) {
	rss := `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:dc="http://purl.org/dc/elements/1.1/">
<channel>
  <item>
    <title>Math Lecturer</title>
    <link>https://www.academiccareers.com/job/111</link>
    <description>Teach mathematics.</description>
    <pubDate>Mon, 15 Mar 2025 00:00:00 GMT</pubDate>
  </item>
  <item>
    <title>CS Assistant Professor</title>
    <link>https://www.academiccareers.com/job/222</link>
    <description>Computer science position.</description>
    <pubDate>Tue, 16 Mar 2025 00:00:00 GMT</pubDate>
  </item>
</channel>
</rss>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(rss))
	}))
	defer srv.Close()

	s := sut.NewWithURLs(srv.Client(), srv.URL)
	jobs, err := s.Scrape(context.Background(), model.ScraperInput{SearchTerm: "computer", ResultsWanted: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job matching 'computer', got %d", len(jobs))
	}
	if jobs[0].Title != "CS Assistant Professor" {
		t.Fatalf("unexpected title: %s", jobs[0].Title)
	}
}

func TestAcademicCareersFailsOn404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNotFound) }))
	s := sut.NewWithURLs(srv.Client(), srv.URL)
	_, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 1})
	srv.Close()
	if err == nil {
		t.Fatal("expected error on 404")
	}
}
