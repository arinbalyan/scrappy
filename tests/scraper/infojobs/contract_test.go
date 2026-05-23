package infojobs_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	sut "github.com/arinbalyan/scrappy/internal/scraper/infojobs"
)

// ─── Test helpers ──────────────────────────────────────────────────────────────

type testValue struct {
	ID    int    `json:"id"`
	Value string `json:"value"`
}

type testCompany struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	URL  string `json:"url"`
}

type testOffer struct {
	ID          string       `json:"id"`
	Title       string       `json:"title"`
	City        string       `json:"city"`
	Company     *testCompany `json:"company"`
	Link        string       `json:"link"`
	Province    *testValue   `json:"province"`
	Description *string      `json:"description"`
	Published   string       `json:"published"`
}

type testResponse struct {
	CurrentPage    int         `json:"currentPage"`
	PageSize       int         `json:"pageSize"`
	TotalPages     int         `json:"totalPages"`
	TotalResults   int         `json:"totalResults"`
	CurrentResults int         `json:"currentResults"`
	Items          []testOffer `json:"items"`
}

// sampleAPIResponse returns a JSON response for a given page with N offers.
func sampleAPIResponse(page, totalPages, count int) string {
	items := make([]testOffer, 0, count)
	for i := 0; i < count; i++ {
		idx := (page-1)*5 + i + 1
		title := fmt.Sprintf("Software Engineer %d", idx)
		desc := "<p>Job description for position " + title + "</p>"
		items = append(items, testOffer{
			ID:    fmt.Sprintf("%d", idx),
			Title: title,
			City:  "Madrid",
			Company: &testCompany{
				ID:   idx,
				Name: "Tech Corp",
				URL:  "https://techcorp.com",
			},
			Link: fmt.Sprintf("https://www.infojobs.net/offer/%d", idx),
			Province: &testValue{
				ID:    28,
				Value: "Madrid",
			},
			Description: &desc,
			Published:   "2026-05-20T10:00:00Z",
		})
	}

	resp := testResponse{
		CurrentPage:    page,
		PageSize:       count,
		TotalPages:     totalPages,
		TotalResults:   totalPages * count,
		CurrentResults: count,
		Items:          items,
	}

	b, _ := json.Marshal(resp)
	return string(b)
}

// ─── Tests ─────────────────────────────────────────────────────────────────────

// TestHappyPath verifies the scraper returns expected jobs from a mock API.
func TestHappyPath(t *testing.T) {
	pageCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pageCount++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// Page 1: 3 items, Page 2: 2 items (no more pages)
		if pageCount == 1 {
			_, _ = w.Write([]byte(sampleAPIResponse(1, 2, 3)))
		} else {
			_, _ = w.Write([]byte(sampleAPIResponse(2, 2, 2)))
		}
	}))
	defer srv.Close()

	s := sut.NewWithURLs(srv.Client(), srv.URL)
	t.Setenv("INFOJOBS_CLIENT_ID", "test-client")
	t.Setenv("INFOJOBS_CLIENT_SECRET", "test-secret")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	jobs, err := s.Scrape(ctx, model.ScraperInput{SearchTerm: "engineer", ResultsWanted: 5})
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if len(jobs) != 5 {
		t.Fatalf("expected 5 jobs, got %d", len(jobs))
	}

	// Verify jobs have correct structure
	for i, j := range jobs {
		if len(j.ID) < 10 || j.ID[:9] != "infojobs-" {
			t.Errorf("job[%d] ID %q does not have infojobs- prefix", i, j.ID)
		}
		if j.Title == "" {
			t.Errorf("job[%d] has empty title", i)
		}
		if j.CompanyName == "" {
			t.Errorf("job[%d] has empty company", i)
		}
	}
}

// TestErrorHandling429 verifies rate-limit errors propagate.
func TestErrorHandling429(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	s := sut.NewWithURLs(srv.Client(), srv.URL)
	t.Setenv("INFOJOBS_CLIENT_ID", "test-client")
	t.Setenv("INFOJOBS_CLIENT_SECRET", "test-secret")

	_, err := s.Scrape(context.Background(), model.ScraperInput{SearchTerm: "x", ResultsWanted: 1})
	if err == nil {
		t.Fatal("expected error for status 429")
	}
}

// TestErrorHandling503 verifies server-error propagation.
func TestErrorHandling503(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	s := sut.NewWithURLs(srv.Client(), srv.URL)
	t.Setenv("INFOJOBS_CLIENT_ID", "test-client")
	t.Setenv("INFOJOBS_CLIENT_SECRET", "test-secret")

	_, err := s.Scrape(context.Background(), model.ScraperInput{SearchTerm: "x", ResultsWanted: 1})
	if err == nil {
		t.Fatal("expected error for status 503")
	}
}

// TestEmptyResponse verifies that an empty items array results in an error.
func TestEmptyResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"currentPage":1,"pageSize":25,"totalPages":0,"totalResults":0,"currentResults":0,"items":[]}`))
	}))
	defer srv.Close()

	s := sut.NewWithURLs(srv.Client(), srv.URL)
	t.Setenv("INFOJOBS_CLIENT_ID", "test-client")
	t.Setenv("INFOJOBS_CLIENT_SECRET", "test-secret")

	_, err := s.Scrape(context.Background(), model.ScraperInput{SearchTerm: "x", ResultsWanted: 1})
	if err == nil {
		t.Fatal("expected error for empty response")
	}
}

// TestContextCancellation verifies that a cancelled context returns an error.
func TestContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Delay to trigger context cancellation
		time.Sleep(500 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(sampleAPIResponse(1, 1, 5)))
	}))
	defer srv.Close()

	s := sut.NewWithURLs(srv.Client(), srv.URL)
	t.Setenv("INFOJOBS_CLIENT_ID", "test-client")
	t.Setenv("INFOJOBS_CLIENT_SECRET", "test-secret")

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := s.Scrape(ctx, model.ScraperInput{SearchTerm: "engineer", ResultsWanted: 5})
	if err == nil {
		t.Fatal("expected error for context cancellation")
	}
}

// TestMissingCredentials returns an error when env vars are not set.
func TestMissingCredentials(t *testing.T) {
	s := sut.New(nil)
	// Do NOT set env vars
	_, err := s.Scrape(context.Background(), model.ScraperInput{SearchTerm: "x", ResultsWanted: 1})
	if err == nil {
		t.Fatal("expected error for missing credentials")
	}
}
