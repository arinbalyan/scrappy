package usajobs

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/arinbalyan/scrappy/internal/model"
)

const testAPIResponse = `{
  "SearchResult": {
    "SearchResultCount": 3,
    "SearchResultCountAll": 3,
    "SearchResultItems": [
      {
        "MatchedObjectId": "job001",
        "MatchedObjectDescriptor": {
          "PositionTitle": "Software Engineer",
          "PositionURI": "https://www.usajobs.gov/job/12345",
          "PositionID": "12345",
          "OrganizationName": "Department of Technology",
          "DepartmentName": "DT",
          "PositionLocation": [
            {"LocationName": "Washington DC", "CountryCode": "US", "CityName": "Washington", "CountrySubDivisionCode": "DC"}
          ],
          "PositionRemuneration": [
            {"MinimumRange": "80000", "MaximumRange": "120000", "RateIntervalCode": "PA", "Description": "Per Year"}
          ],
          "PublicationStartDate": "2026-05-20T10:00:00Z",
          "ApplicationCloseDate": "2026-06-20",
          "QualificationSummary": "<p>Must have 5 years experience.</p>",
          "UserArea": {
            "Details": {
              "JobSummary": "Build and maintain government software systems.",
              "MajorDuties": ["Design system architecture", "Write unit tests", "Review code"]
            }
          }
        }
      },
      {
        "MatchedObjectId": "job002",
        "MatchedObjectDescriptor": {
          "PositionTitle": "Data Analyst",
          "PositionURI": "https://www.usajobs.gov/job/67890",
          "PositionID": "67890",
          "OrganizationName": "Bureau of Statistics",
          "DepartmentName": "BS",
          "PositionLocation": [
            {"LocationName": "New York, NY", "CountryCode": "US", "CityName": "New York", "CountrySubDivisionCode": "NY"}
          ],
          "PositionRemuneration": [
            {"MinimumRange": "60000", "MaximumRange": "90000", "RateIntervalCode": "PA", "Description": "Per Year"}
          ],
          "PublicationStartDate": "2026-05-18T08:00:00Z",
          "ApplicationCloseDate": "2026-06-18",
          "QualificationSummary": "Data analysis skills required.",
          "UserArea": {
            "Details": {
              "JobSummary": "Analyze government data sets.",
              "MajorDuties": ["Clean data", "Generate reports"]
            }
          }
        }
      },
      {
        "MatchedObjectId": "empty",
        "MatchedObjectDescriptor": {
          "PositionTitle": "",
          "PositionURI": "",
          "PositionID": "",
          "UserArea": null
        }
      }
    ]
  }
}`

func TestScraper_SiteName(t *testing.T) {
	s := New(nil)
	if got := s.SiteName(); got != model.SiteUSAJobs {
		t.Errorf("SiteName() = %q, want %q", got, model.SiteUSAJobs)
	}
}

func TestScraper_IsConfigured(t *testing.T) {
	s1 := New(nil)
	if s1.IsConfigured() {
		t.Error("New() without env vars should report not configured")
	}

	s2 := NewWithCredentials(nil, "", "test-key", "test-email")
	if !s2.IsConfigured() {
		t.Error("NewWithCredentials() should report configured")
	}
}

func TestScraper_Scrape(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(testAPIResponse))
	}))
	defer ts.Close()

	s := NewWithCredentials(nil, ts.URL, "test-key", "test@example.com")
	jobs, err := s.Scrape(context.Background(), model.ScraperInput{
		ResultsWanted: 25,
		SearchTerm:    "engineer",
	})
	if err != nil {
		t.Fatalf("Scrape() returned error: %v", err)
	}

	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(jobs))
	}

	// Job 0
	j0 := jobs[0]
	if j0.Title != "Software Engineer" {
		t.Errorf("job[0].Title = %q, want %q", j0.Title, "Software Engineer")
	}
	if j0.CompanyName != "Department of Technology" {
		t.Errorf("job[0].CompanyName = %q, want %q", j0.CompanyName, "Department of Technology")
	}
	if j0.Site != string(model.SiteUSAJobs) {
		t.Errorf("job[0].Site = %q, want %q", j0.Site, model.SiteUSAJobs)
	}
	if j0.Location.City != "Washington" {
		t.Errorf("job[0].Location.City = %q, want %q", j0.Location.City, "Washington")
	}
	if j0.Location.State != "DC" {
		t.Errorf("job[0].Location.State = %q, want %q", j0.Location.State, "DC")
	}
	if j0.Location.Country != "US" {
		t.Errorf("job[0].Location.Country = %q, want %q", j0.Location.Country, "US")
	}
	if j0.Compensation == nil {
		t.Fatal("job[0].Compensation is nil")
	}
	if j0.Compensation.Interval != "yearly" {
		t.Errorf("job[0].Compensation.Interval = %q, want yearly", j0.Compensation.Interval)
	}
	if j0.Description == "" {
		t.Error("job[0].Description is empty")
	}
	if !strings.Contains(j0.Description, "Major Duties:") {
		t.Errorf("job[0].Description missing Major Duties section")
	}
	if j0.DatePosted == nil {
		t.Error("job[0].DatePosted is nil")
	}

	// Job 1
	j1 := jobs[1]
	if j1.Title != "Data Analyst" {
		t.Errorf("job[1].Title = %q, want %q", j1.Title, "Data Analyst")
	}
	if j1.CompanyName != "Bureau of Statistics" {
		t.Errorf("job[1].CompanyName = %q, want %q", j1.CompanyName, "Bureau of Statistics")
	}
}

func TestScraper_Scrape_NoCredentials(t *testing.T) {
	s := New(nil)
	_, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 10})
	if err == nil {
		t.Fatal("expected error when no credentials configured, got nil")
	}
}

func TestScraper_Scrape_HTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	s := NewWithCredentials(nil, ts.URL, "key", "email@test.com")
	_, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 10})
	if err == nil {
		t.Fatal("expected error for 500 status, got nil")
	}
}

func TestScraper_Scrape_EmptyResponse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"SearchResult": {"SearchResultCount": 0, "SearchResultItems": []}}`))
	}))
	defer ts.Close()

	s := NewWithCredentials(nil, ts.URL, "key", "email@test.com")
	_, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 10})
	if err == nil {
		t.Fatal("expected error for empty response, got nil")
	}
}

func TestScraper_Scrape_429(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer ts.Close()

	s := NewWithCredentials(nil, ts.URL, "key", "email@test.com")
	_, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 10})
	if err == nil {
		t.Fatal("expected error for 429 status, got nil")
	}
}

func TestMapCompensation(t *testing.T) {
	tests := []struct {
		r        apiRemuneration
		wantIntv model.CompensationInterval
		wantMin  float64
		wantMax  float64
	}{
		{apiRemuneration{MinimumRange: "50000", MaximumRange: "70000", Description: "Per Year"}, model.CompensationInterval("yearly"), 50000, 70000},
		{apiRemuneration{MinimumRange: "30", MaximumRange: "50", Description: "Per Hour"}, model.CompensationInterval("hourly"), 30, 50},
		{apiRemuneration{MinimumRange: "0", MaximumRange: "0"}, model.CompensationInterval("yearly"), 0, 0},
		{apiRemuneration{MinimumRange: "", MaximumRange: "abc"}, model.CompensationInterval(""), 0, 0},
	}

	for _, tt := range tests {
		comp := mapCompensation(tt.r)
		if tt.wantMin == 0 && tt.wantMax == 0 && tt.wantIntv == "" {
			if comp != nil {
				t.Errorf("mapCompensation(%+v) = %+v, want nil", tt.r, comp)
			}
			continue
		}
		if comp == nil {
			t.Errorf("mapCompensation(%+v) = nil, want non-nil", tt.r)
			continue
		}
		if comp.Interval != tt.wantIntv {
			t.Errorf("mapCompensation(%+v).Interval = %q, want %q", tt.r, comp.Interval, tt.wantIntv)
		}
		if comp.MinAmount != nil && *comp.MinAmount != tt.wantMin {
			t.Errorf("mapCompensation(%+v).MinAmount = %f, want %f", tt.r, *comp.MinAmount, tt.wantMin)
		}
	}
}
