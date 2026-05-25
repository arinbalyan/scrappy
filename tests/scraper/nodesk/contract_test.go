package nodesk_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	sut "github.com/arinbalyan/scrappy/internal/scraper/nodesk"
)

func TestNoDeskFailsOnEmptyResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	s := sut.NewWithAPIURL(srv.Client(), srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	jobs, err := s.Scrape(ctx, model.ScraperInput{SearchTerm: "developer", ResultsWanted: 5})
	if err == nil && len(jobs) == 0 {
		t.Fatal("expected non-empty jobs or explicit error for empty upstream response")
	}
}

func TestNoDeskFailsOn429And503(t *testing.T) {
	for _, status := range []int{http.StatusTooManyRequests, http.StatusServiceUnavailable} {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(status)
		}))
		s := sut.NewWithAPIURL(srv.Client(), srv.URL)
		_, err := s.Scrape(ctx, model.ScraperInput{SearchTerm: "developer", ResultsWanted: 5})
		srv.Close()
		cancel()
		if err == nil {
			t.Fatalf("expected error for status %d", status)
		}
	}
}

func TestNoDeskScraper_ScrapeBasic(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		body := `[
			{
				"id": 101,
				"title": "Senior Go Developer",
				"company": "TechCorp",
				"company_logo": "https://example.com/logo.png",
				"url": "https://nodesk.co/jobs/101",
				"description": "Build backend services in Go",
				"location": "Remote",
				"tags": ["go", "kubernetes", "postgresql"],
				"published_at": "2026-05-20T10:00:00Z"
			},
			{
				"id": "202",
				"title": "Frontend Engineer",
				"company": "WebStudio",
				"url": "https://nodesk.co/jobs/202",
				"description": "React and TypeScript developer",
				"location": "Remote",
				"tags": ["react", "typescript"],
				"date": "2026-05-19"
			}
		]`
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	s := sut.NewWithAPIURL(srv.Client(), srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	jobs, err := s.Scrape(ctx, model.ScraperInput{ResultsWanted: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(jobs))
	}

	// First job — numeric ID, full fields, published_at
	j1 := jobs[0]
	if j1.ID != "nodesk-101" {
		t.Fatalf("expected ID 'nodesk-101', got %q", j1.ID)
	}
	if j1.Title != "Senior Go Developer" {
		t.Fatalf("expected title 'Senior Go Developer', got %q", j1.Title)
	}
	if j1.CompanyName != "TechCorp" {
		t.Fatalf("expected company 'TechCorp', got %q", j1.CompanyName)
	}
	if j1.CompanyLogoURL != "https://example.com/logo.png" {
		t.Fatalf("expected logo URL, got %q", j1.CompanyLogoURL)
	}
	if j1.JobURL != "https://nodesk.co/jobs/101" {
		t.Fatalf("expected job URL, got %q", j1.JobURL)
	}
	if j1.Description != "Build backend services in Go" {
		t.Fatalf("expected description, got %q", j1.Description)
	}
	if j1.Location.City != "Remote" {
		t.Fatalf("expected location 'Remote', got %q", j1.Location.City)
	}
	if !j1.IsRemote {
		t.Fatal("expected IsRemote = true (hardcoded)")
	}
	if j1.DatePosted == nil {
		t.Fatal("expected date from published_at")
	}
	if !j1.DatePosted.Equal(time.Date(2026, 5, 20, 10, 0, 0, 0, time.UTC)) {
		t.Fatalf("expected 2026-05-20T10:00:00Z, got %v", j1.DatePosted)
	}
	if len(j1.Skills) != 3 || j1.Skills[0] != "go" || j1.Skills[1] != "kubernetes" {
		t.Fatalf("unexpected skills: %v", j1.Skills)
	}

	// Second job — string ID, optional fields, uses 'date' instead of published_at
	j2 := jobs[1]
	if j2.ID != "nodesk-202" {
		t.Fatalf("expected ID 'nodesk-202', got %q", j2.ID)
	}
	if j2.Title != "Frontend Engineer" {
		t.Fatalf("expected title 'Frontend Engineer', got %q", j2.Title)
	}
	if j2.CompanyName != "WebStudio" {
		t.Fatalf("expected company 'WebStudio', got %q", j2.CompanyName)
	}
	if j2.JobURL != "https://nodesk.co/jobs/202" {
		t.Fatalf("expected job URL, got %q", j2.JobURL)
	}
	if j2.Description != "React and TypeScript developer" {
		t.Fatalf("expected description, got %q", j2.Description)
	}
	if j2.Location.City != "Remote" {
		t.Fatalf("expected location 'Remote', got %q", j2.Location.City)
	}
	if !j2.IsRemote {
		t.Fatal("expected IsRemote = true (hardcoded)")
	}
	if j2.DatePosted == nil {
		t.Fatal("expected date from 'date' field")
	}
	if !j2.DatePosted.Equal(time.Date(2026, 5, 19, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("expected 2026-05-19, got %v", j2.DatePosted)
	}
	if len(j2.Skills) != 2 || j2.Skills[0] != "react" || j2.Skills[1] != "typescript" {
		t.Fatalf("unexpected skills: %v", j2.Skills)
	}
}

func TestNoDeskScraper_HandlesFlexibleResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		body := `{
			"jobs": [
				{
					"id": "flex1",
					"title": "Product Designer",
					"company": "Design Co",
					"url": "https://nodesk.co/jobs/flex1",
					"tags": ["ui", "ux"]
				},
				{
					"id": "flex2",
					"title": "DevOps Engineer",
					"company": "CloudOps Inc",
					"url": "https://nodesk.co/jobs/flex2"
				}
			]
		}`
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	s := sut.NewWithAPIURL(srv.Client(), srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	jobs, err := s.Scrape(ctx, model.ScraperInput{ResultsWanted: 10})
	if err != nil {
		t.Fatalf("unexpected error with flexible response: %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs from wrapper format, got %d", len(jobs))
	}
	if jobs[0].ID != "nodesk-flex1" {
		t.Fatalf("expected ID 'nodesk-flex1', got %q", jobs[0].ID)
	}
	if jobs[1].ID != "nodesk-flex2" {
		t.Fatalf("expected ID 'nodesk-flex2', got %q", jobs[1].ID)
	}
	if len(jobs[0].Skills) != 2 || jobs[0].Skills[0] != "ui" {
		t.Fatalf("expected skills ['ui', 'ux'], got %v", jobs[0].Skills)
	}
	if !jobs[0].IsRemote {
		t.Fatal("expected IsRemote = true (hardcoded)")
	}
	if !jobs[1].IsRemote {
		t.Fatal("expected IsRemote = true (hardcoded)")
	}
}

func TestNoDeskScraper_RemoteHardcoded(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		body := `[
			{
				"id": "r1",
				"title": "Remote Role",
				"company": "RemoteCo",
				"url": "https://nodesk.co/r1",
				"is_remote": false
			}
		]`
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	s := sut.NewWithAPIURL(srv.Client(), srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	jobs, err := s.Scrape(ctx, model.ScraperInput{ResultsWanted: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
	if !jobs[0].IsRemote {
		t.Fatal("expected IsRemote = true even when upstream says false")
	}
}

func TestNoDeskScraper_FiltersBySearchTerm(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		body := `[
			{"id": 1, "title": "Go Developer", "company": "A", "url": "https://a.com/1"},
			{"id": 2, "title": "Frontend Engineer", "company": "B", "url": "https://b.com/2"},
			{"id": 3, "title": "Senior Go Engineer", "company": "C", "url": "https://c.com/3"},
			{"id": 4, "title": "DevOps Engineer", "company": "D", "url": "https://d.com/4"},
			{"id": 5, "title": "Product Designer", "company": "E", "url": "https://e.com/5"}
		]`
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	s := sut.NewWithAPIURL(srv.Client(), srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	jobs, err := s.Scrape(ctx, model.ScraperInput{SearchTerm: "Go", ResultsWanted: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs matching 'Go', got %d", len(jobs))
	}
	if jobs[0].Title != "Go Developer" {
		t.Fatalf("expected 'Go Developer', got %q", jobs[0].Title)
	}
	if jobs[1].Title != "Senior Go Engineer" {
		t.Fatalf("expected 'Senior Go Engineer', got %q", jobs[1].Title)
	}
}

func TestNoDeskScraper_RespectsResultsWanted(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		body := `[
			{"id": 1, "title": "Engineer A", "company": "A", "url": "https://a.com/1"},
			{"id": 2, "title": "Engineer B", "company": "B", "url": "https://b.com/2"},
			{"id": 3, "title": "Engineer C", "company": "C", "url": "https://c.com/3"},
			{"id": 4, "title": "Engineer D", "company": "D", "url": "https://d.com/4"},
			{"id": 5, "title": "Engineer E", "company": "E", "url": "https://e.com/5"}
		]`
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	s := sut.NewWithAPIURL(srv.Client(), srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	jobs, err := s.Scrape(ctx, model.ScraperInput{ResultsWanted: 3})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(jobs) != 3 {
		t.Fatalf("expected 3 jobs, got %d", len(jobs))
	}
}
