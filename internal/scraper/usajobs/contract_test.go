package usajobs

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/arinbalyan/scrappy/internal/model"
)

func TestScraper_SiteName(t *testing.T) {
	s := New(nil)
	if got := s.SiteName(); got != model.SiteUSAJobs {
		t.Errorf("SiteName() = %q, want %q", got, model.SiteUSAJobs)
	}
}

func TestScraper_Scrape(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		resp := usaJobsResponse{
			SearchResult: &searchResult{
				SearchResultCount:    2,
				SearchResultCountAll: 2,
				SearchResultItems: []usaJobsItem{
					{
						MatchedObjectId: "JOB001",
						MatchedObjectDescriptor: &jobDescriptor{
							PositionTitle: "Software Engineer",
							PositionURI:   "https://www.usajobs.gov/job/001",
							OrganizationName: "Dept of Technology",
							PositionLocation: []usaJobsLocation{
								{CityName: "Washington", CountrySubDivisionCode: "DC", CountryCode: "USA"},
							},
							PositionRemuneration: []usaJobsRemuneration{
								{MinimumRange: "80000", MaximumRange: "120000", RateIntervalCode: "Per Year", Description: "Per Year"},
							},
							PublicationStartDate: "2026-05-20T00:00:00",
							UserArea: &userArea{
								Details: &userDetails{
									JobSummary:  "Build and maintain government software systems.",
									MajorDuties: []string{"Write code", "Review PRs", "Deploy services"},
								},
							},
						},
					},
					{
						MatchedObjectId: "JOB002",
						MatchedObjectDescriptor: &jobDescriptor{
							PositionTitle: "Data Analyst",
							PositionURI:   "https://www.usajobs.gov/job/002",
							OrganizationName: "Dept of Data",
							PositionLocation: []usaJobsLocation{
								{CityName: "Remote"},
							},
							PublicationStartDate: "2026-05-21",
							QualificationSummary: "Must have strong analytical skills.",
						},
					},
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	s := NewWithAPIURL(nil, ts.URL)
	jobs, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 25})
	if err != nil {
		t.Fatalf("Scrape() returned error: %v", err)
	}

	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(jobs))
	}

	// Job 0
	j0 := jobs[0]
	if j0.ID != "usajobs-JOB001" {
		t.Errorf("job[0].ID = %q, want %q", j0.ID, "usajobs-JOB001")
	}
	if j0.Title != "Software Engineer" {
		t.Errorf("job[0].Title = %q, want %q", j0.Title, "Software Engineer")
	}
	if j0.CompanyName != "Dept of Technology" {
		t.Errorf("job[0].CompanyName = %q, want %q", j0.CompanyName, "Dept of Technology")
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
	if j0.Compensation == nil {
		t.Fatal("job[0].Compensation is nil")
	}
	if j0.Compensation.Currency != "USD" {
		t.Errorf("job[0].Compensation.Currency = %q", j0.Compensation.Currency)
	}
	if j0.DatePosted == nil {
		t.Error("job[0].DatePosted is nil")
	}
	if !strings.Contains(j0.Description, "Build and maintain") {
		t.Errorf("job[0].Description missing job summary: %q", j0.Description)
	}

	// Job 1
	j1 := jobs[1]
	if j1.ID != "usajobs-JOB002" {
		t.Errorf("job[1].ID = %q, want %q", j1.ID, "usajobs-JOB002")
	}
	if j1.Title != "Data Analyst" {
		t.Errorf("job[1].Title = %q, want %q", j1.Title, "Data Analyst")
	}
	if j1.Compensation != nil {
		t.Error("job[1].Compensation should be nil (no remuneration data)")
	}
}

func TestScraper_Scrape_HTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	s := NewWithAPIURL(nil, ts.URL)
	_, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 25})
	if err == nil {
		t.Fatal("expected error for 500 status, got nil")
	}
}

func TestScraper_Scrape_EmptyResponse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, `{"SearchResult":{"SearchResultCount":0,"SearchResultItems":[]}}`)
	}))
	defer ts.Close()

	s := NewWithAPIURL(nil, ts.URL)
	_, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 25})
	if err == nil {
		t.Fatal("expected error for empty results, got nil")
	}
}

func TestScraper_Scrape_MissingDescriptor(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		resp := usaJobsResponse{
			SearchResult: &searchResult{
				SearchResultCount:    1,
				SearchResultItems: []usaJobsItem{
					{MatchedObjectId: "JOB003"},
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	s := NewWithAPIURL(nil, ts.URL)
	_, err := s.Scrape(context.Background(), model.ScraperInput{ResultsWanted: 25})
	if err == nil {
		t.Fatal("expected error for job with no descriptor (no parseable jobs), got nil")
	}
}

func TestBuildURL(t *testing.T) {
	u1 := buildURL(apiURL, "developer", "", 0, 1, 25)
	if !strings.Contains(u1, "Keyword=developer") {
		t.Errorf("URL should contain keyword: %s", u1)
	}
	if !strings.Contains(u1, "Page=1") {
		t.Errorf("URL should contain page: %s", u1)
	}

	u2 := buildURL(apiURL, "", "New York", 48, 2, 50)
	if !strings.Contains(u2, "LocationName=New+York") {
		t.Errorf("URL should contain location: %s", u2)
	}
	if !strings.Contains(u2, "DatePosted=2") {
		t.Errorf("URL should contain DatePosted: %s", u2)
	}
}

func TestNewWithAPIURL_EmptyEndpoint(t *testing.T) {
	s1 := NewWithAPIURL(nil, "")
	s2 := New(nil)
	if s1.apiURL != s2.apiURL {
		t.Errorf("empty endpoint should not override API URL")
	}
}
