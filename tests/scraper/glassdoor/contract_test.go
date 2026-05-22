package glassdoor_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	sut "github.com/arinbalyan/scrappy/internal/scraper/glassdoor"
)

// sampleHomepage is a minimal Glassdoor homepage HTML containing a CSRF token.
const sampleHomepage = `<!DOCTYPE html>
<html>
<head><title>Glassdoor</title></head>
<body>
<script>
  var gdCSRF = "test-csrf-value-12345";
</script>
</body>
</html>`

// sampleGraphQLResponse is a minimal Glassdoor GraphQL response with one job listing.
const sampleGraphQLResponse = `{
  "data": {
    "jobListings": {
      "paginationCursors": [
        { "cursor": "page2cursor", "pageNumber": 2 }
      ],
      "jobListings": [
        {
          "jobview": {
            "header": {
              "adOrderId": 1001,
              "ageInDays": 2,
              "employerNameFromSearch": "Acme Corp",
              "jobLink": "/job-listing/software-engineer-1001",
              "jobTitleText": "Software Engineer",
              "locationName": "San Francisco, CA",
              "locationType": "N",
              "payCurrency": "USD",
              "payPeriod": "YEARLY",
              "payPeriodAdjustedPay": {
                "p10": 120000,
                "p50": 150000,
                "p90": 180000
              },
              "rating": 4.2,
              "seoJobLink": "/job-listing/software-engineer-seo-1001",
              "sponsored": false,
              "easyApply": false,
              "employer": {
                "id": 500,
                "name": "Acme Corp",
                "shortName": "Acme"
              }
            },
            "job": {
              "descriptionFragments": [
                "We are looking for a Software Engineer to join our team.",
                "You will work on exciting projects."
              ],
              "listingId": 999001
            },
            "overview": {
              "id": 500,
              "name": "Acme Corporation",
              "shortName": "Acme",
              "squareLogoUrl": "https://logo.example.com/acme.png"
            }
          }
        },
        {
          "jobview": {
            "header": {
              "adOrderId": 1002,
              "ageInDays": 0,
              "employerNameFromSearch": "TechStartup Inc",
              "jobLink": "/job-listing/remote-engineer-1002",
              "jobTitleText": "Senior Backend Engineer",
              "locationName": "Remote, US",
              "locationType": "S",
              "payCurrency": "USD",
              "payPeriod": "YEARLY",
              "payPeriodAdjustedPay": {
                "p10": 150000,
                "p50": 180000,
                "p90": 220000
              },
              "rating": null,
              "sponsored": true,
              "easyApply": true,
              "employer": {
                "id": 501,
                "name": "TechStartup Inc",
                "shortName": "TechStartup"
              }
            },
            "job": {
              "descriptionFragments": [
                "Remote-first senior backend role."
              ],
              "listingId": 999002
            },
            "overview": {
              "id": 501,
              "name": "TechStartup Inc",
              "shortName": "TechStartup",
              "squareLogoUrl": "https://logo.example.com/techstartup.png"
            }
          }
        }
      ]
    }
  }
}`

func TestGlassdoorParsesJobs(t *testing.T) {
	var postCount int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(sampleHomepage))
			return
		}
		postCount++
		// First POST returns jobs; subsequent POSTs return empty to end pagination.
		if postCount > 1 {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":{"jobListings":{"paginationCursors":[],"jobListings":[]}}}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(sampleGraphQLResponse))
	}))
	defer srv.Close()

	s := sut.NewWithURLs(srv.Client(), srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	jobs, err := s.Scrape(ctx, model.ScraperInput{
		SearchTerm:    "software engineer",
		ResultsWanted: 10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(jobs) == 0 {
		t.Fatal("expected at least one job")
	}

	// Verify first job fields.
	j1 := jobs[0]
	if j1.ID != "gd-1001" {
		t.Errorf("expected ID 'gd-1001', got %q", j1.ID)
	}
	if !strings.Contains(j1.Title, "Software Engineer") {
		t.Errorf("expected title containing 'Software Engineer', got %q", j1.Title)
	}
	if j1.CompanyName != "Acme Corp" {
		t.Errorf("expected company 'Acme Corp', got %q", j1.CompanyName)
	}
	if !strings.HasPrefix(j1.JobURL, srv.URL) {
		t.Errorf("expected absolute URL, got %q", j1.JobURL)
	}
	if j1.Location.City != "San Francisco" {
		t.Errorf("expected city 'San Francisco', got %q", j1.Location.City)
	}
	if j1.Location.State != "CA" {
		t.Errorf("expected state 'CA', got %q", j1.Location.State)
	}
	if j1.Compensation == nil {
		t.Fatal("expected compensation to be parsed")
	}
	if j1.Compensation.Currency != "USD" {
		t.Errorf("expected currency 'USD', got %q", j1.Compensation.Currency)
	}
	if j1.Compensation.MinAmount == nil || *j1.Compensation.MinAmount != 120000 {
		t.Errorf("expected min amount 120000, got %v", j1.Compensation.MinAmount)
	}
	if j1.Compensation.MaxAmount == nil || *j1.Compensation.MaxAmount != 180000 {
		t.Errorf("expected max amount 180000, got %v", j1.Compensation.MaxAmount)
	}
	if j1.Compensation.Interval != model.IntervalYearly {
		t.Errorf("expected yearly interval, got %v", j1.Compensation.Interval)
	}
	if j1.IsRemote {
		t.Error("expected non-remote for first job")
	}
	if j1.DatePosted == nil {
		t.Error("expected datePosted to be set")
	} else {
		// Should be ~2 days ago.
		daysAgo := int(time.Since(*j1.DatePosted).Hours() / 24)
		if daysAgo != 2 {
			t.Logf("note: datePosted ~%d days ago (expected ~2)", daysAgo)
		}
	}
	if j1.ApplyMethod != "external_url" {
		t.Errorf("expected apply_method external_url, got %q", j1.ApplyMethod)
	}

	// Verify second job (remote, easy apply, today).
	if len(jobs) < 2 {
		t.Fatal("expected at least 2 jobs")
	}
	j2 := jobs[1]
	if j2.ID != "gd-1002" {
		t.Errorf("expected ID 'gd-1002', got %q", j2.ID)
	}
	if !j2.IsRemote {
		t.Error("expected second job to be remote")
	}
	if j2.ApplyMethod != "easy_apply" {
		t.Errorf("expected apply_method easy_apply, got %q", j2.ApplyMethod)
	}
	if j2.DatePosted != nil {
		hoursAgo := time.Since(*j2.DatePosted).Hours()
		if hoursAgo > 24 {
			t.Errorf("expected job posted today (ageInDays=0), but got %0.f hours ago", hoursAgo)
		}
	}
}

func TestGlassdoorFailsOn429And503(t *testing.T) {
	for _, status := range []int{http.StatusTooManyRequests, http.StatusServiceUnavailable} {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		callCount := 0
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount++
			if r.Method == http.MethodGet {
				// Return homepage with CSRF token.
				w.Header().Set("Content-Type", "text/html")
				_, _ = w.Write([]byte(sampleHomepage))
				return
			}
			// POST → return error status.
			w.WriteHeader(status)
		}))
		s := sut.NewWithURLs(srv.Client(), srv.URL)
		_, err := s.Scrape(ctx, model.ScraperInput{
			SearchTerm:    "software engineer",
			ResultsWanted: 5,
		})
		srv.Close()
		if err == nil {
			t.Fatalf("expected error for status %d", status)
		}
	}
}
