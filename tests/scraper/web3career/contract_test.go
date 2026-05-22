package web3career_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	sut "github.com/arinbalyan/scrappy/internal/scraper/web3career"
)

const sampleJobsPayload = `[
  {
    "id": 12345,
    "title": "Senior Solidity Engineer",
    "company": "Ethereum Foundation",
    "company_logo": "https://web3.career/logo.png",
    "url": "https://web3.career/job/12345",
    "description": "Build <b>smart contracts</b> for the next generation of DeFi.",
    "location": "Remote",
    "tags": ["solidity", "ethereum", "defi"],
    "salary_min": 150000,
    "salary_max": 250000,
    "salary_currency": "USD",
    "date_posted": "2026-05-20T12:00:00Z",
    "is_remote": true
  },
  {
    "id": 12346,
    "title": "Blockchain Backend Developer",
    "company": "Chainlink Labs",
    "company_logo": "https://web3.career/chainlink.png",
    "url": "https://web3.career/job/12346",
    "description": "Build oracle infrastructure for Web3.",
    "location": "San Francisco, CA",
    "tags": ["blockchain", "go", "rust"],
    "salary_min": 130000,
    "salary_max": 200000,
    "salary_currency": "USD",
    "date_posted": "2026-05-19T08:00:00Z",
    "remote": false
  },
  {
    "id": 12347,
    "title": "Web3 Frontend Engineer",
    "company": "Uniswap",
    "url": "https://web3.career/job/12347",
    "description": "Build React dApp interfaces.",
    "location": "New York, NY",
    "tags": ["react", "web3", "typescript"],
    "salary_min": 120000,
    "salary_max": 180000,
    "salary_currency": "USD",
    "created_at": "2026-05-18T10:30:00Z",
    "is_remote": false
  }
]`

const sampleObjectPayload = `{
  "data": [
    {
      "id": "job-a",
      "title": "Rust Smart Contract Developer",
      "company": "Solana Foundation",
      "url": "https://web3.career/job/job-a",
      "description": "Build Solana programs.",
      "location": "Remote",
      "tags": ["rust", "solana"],
      "salary_min": 160000,
      "salary_max": 220000,
      "salary_currency": "USD",
      "date_posted": "2026-05-21T00:00:00Z",
      "is_remote": true
    }
  ]
}`

func TestWeb3CareerHappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("token") != "public" {
			t.Errorf("expected token=public param, got %s", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(sampleJobsPayload))
	}))
	defer srv.Close()

	s := sut.NewWithAPIURL(srv.Client(), srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	jobs, err := s.Scrape(ctx, model.ScraperInput{SearchTerm: "solidity", ResultsWanted: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job (search filtered), got %d", len(jobs))
	}
	if jobs[0].Title != "Senior Solidity Engineer" {
		t.Errorf("title = %q, want %q", jobs[0].Title, "Senior Solidity Engineer")
	}
	if jobs[0].CompanyName != "Ethereum Foundation" {
		t.Errorf("company = %q, want %q", jobs[0].CompanyName, "Ethereum Foundation")
	}
	if jobs[0].JobURL != "https://web3.career/job/12345" {
		t.Errorf("job_url = %q", jobs[0].JobURL)
	}
	if !jobs[0].IsRemote {
		t.Errorf("expected is_remote = true")
	}
	if jobs[0].Compensation == nil {
		t.Fatal("expected compensation to be set")
	}
	if jobs[0].Skills == nil || len(jobs[0].Skills) != 3 {
		t.Errorf("expected 3 skills, got %v", jobs[0].Skills)
	}
}

func TestWeb3CareerHappyPathNoFilter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(sampleJobsPayload))
	}))
	defer srv.Close()

	s := sut.NewWithAPIURL(srv.Client(), srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	jobs, err := s.Scrape(ctx, model.ScraperInput{ResultsWanted: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(jobs) != 3 {
		t.Fatalf("expected 3 jobs, got %d", len(jobs))
	}
}

func TestWeb3CareerHappyPathObjectResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(sampleObjectPayload))
	}))
	defer srv.Close()

	s := sut.NewWithAPIURL(srv.Client(), srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	jobs, err := s.Scrape(ctx, model.ScraperInput{ResultsWanted: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job from object response, got %d", len(jobs))
	}
	if jobs[0].Title != "Rust Smart Contract Developer" {
		t.Errorf("title = %q", jobs[0].Title)
	}
}

func TestWeb3CareerErrorHandling429(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	s := sut.NewWithAPIURL(srv.Client(), srv.URL)
	_, err := s.Scrape(context.Background(), model.ScraperInput{SearchTerm: "engineer", ResultsWanted: 5})
	if err == nil {
		t.Fatal("expected error for 429 status, got nil")
	}
}

func TestWeb3CareerErrorHandling503(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	s := sut.NewWithAPIURL(srv.Client(), srv.URL)
	_, err := s.Scrape(context.Background(), model.ScraperInput{SearchTerm: "engineer", ResultsWanted: 5})
	if err == nil {
		t.Fatal("expected error for 503 status, got nil")
	}
}

func TestWeb3CareerEmptyResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	s := sut.NewWithAPIURL(srv.Client(), srv.URL)
	_, err := s.Scrape(context.Background(), model.ScraperInput{SearchTerm: "engineer", ResultsWanted: 5})
	if err == nil {
		t.Fatal("expected error for empty response, got nil")
	}
}

func TestWeb3CareerContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // immediately cancel

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(sampleJobsPayload))
	}))
	defer srv.Close()

	s := sut.NewWithAPIURL(srv.Client(), srv.URL)
	_, err := s.Scrape(ctx, model.ScraperInput{SearchTerm: "engineer", ResultsWanted: 3})
	if err == nil {
		t.Fatal("expected error for cancelled context, got nil")
	}
}
