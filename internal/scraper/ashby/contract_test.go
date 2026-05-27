package ashby

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/arinbalyan/scrappy/internal/model"
)

func f64(v float64) *float64 { return &v }

func TestScraper_SiteName(t *testing.T) {
	s := New(nil)
	if got := s.SiteName(); got != model.SiteAshby {
		t.Errorf("SiteName() = %q, want %q", got, model.SiteAshby)
	}
}

func TestScraper_Scrape(t *testing.T) {
	isRemote := true
	isListed := true
	notListed := false

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := ashbyResponse{
			Jobs: []ashbyJob{
				{
					ID:               "job-1",
					Title:            "Software Engineer",
					DepartmentName:   "Engineering",
					EmploymentType:   "Full-Time",
					IsRemote:         &isRemote,
					IsListed:         &isListed,
					PublishedDate:    "2025-01-15",
					DescriptionPlain: "Build awesome software",
					Address: &ashbyAddress{
						PostalAddress: &ashbyPostalAddress{
							AddressLocality: "San Francisco",
							AddressRegion:   "CA",
							AddressCountry:  "US",
						},
					},
				},
				{
					ID:               "job-2",
					Title:            "Product Manager",
					IsListed:         &isListed,
					EmploymentType:   "Full-Time",
					Location:         "New York, NY",
					PublishedDate:    "2025-01-14",
					DescriptionHTML:  "<p>Manage products</p>",
					Compensation: &ashbyCompensation{
						CompensationComponents: []ashbyCompensationComponent{
							{
								CompensationType: "Salary",
								Tiers: []ashbyCompensationTier{
									{
										TierFloor:   f64(100000.0),
										TierCeiling: f64(150000.0),
										Currency:    "USD",
										Interval:    "yearly",
									},
								},
							},
						},
					},
				},
				{
					ID:       "job-3",
					Title:    "Intern",
					IsListed: &notListed,
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	s := NewWithAPIURL(nil, ts.URL)
	result, err := s.Scrape(context.Background(), model.ScraperInput{
		SearchTerm:    "acme",
		ResultsWanted: 25,
	})
	if err != nil {
		t.Fatalf("Scrape() returned error: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(result))
	}

	j1 := result[0]
	if !strings.HasPrefix(j1.ID, "ashby-") {
		t.Errorf("job[0].ID = %q, expected ashby- prefix", j1.ID)
	}
	if j1.Title != "Software Engineer" {
		t.Errorf("job[0].Title = %q", j1.Title)
	}
	if j1.Department != "Engineering" {
		t.Errorf("job[0].Department = %q", j1.Department)
	}
	if j1.JobType != "fulltime" {
		t.Errorf("job[0].JobType = %q", j1.JobType)
	}
	if !j1.IsRemote {
		t.Error("job[0].IsRemote should be true")
	}
	if j1.Location.City != "San Francisco" {
		t.Errorf("job[0].Location.City = %q", j1.Location.City)
	}
	if j1.DatePosted == nil {
		t.Error("job[0].DatePosted is nil")
	}

	j2 := result[1]
	if j2.Title != "Product Manager" {
		t.Errorf("job[1].Title = %q", j2.Title)
	}
	if j2.Location.City != "New York, NY" {
		t.Errorf("job[1].Location.City = %q", j2.Location.City)
	}
	if j2.Compensation == nil {
		t.Fatal("job[1].Compensation is nil")
	}
	if j2.Compensation.Currency != "USD" {
		t.Errorf("job[1].Compensation.Currency = %q", j2.Compensation.Currency)
	}
	if j2.Compensation.MinAmount == nil || *j2.Compensation.MinAmount != 100000.0 {
		t.Errorf("job[1].Compensation.MinAmount = %v", j2.Compensation.MinAmount)
	}
}

func TestScraper_Scrape_NoSeeds(t *testing.T) {
	s := New(nil)
	_, err := s.Scrape(context.Background(), model.ScraperInput{})
	if err == nil {
		t.Fatal("expected error for no seeds, got nil")
	}
}

func TestScraper_Scrape_HTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	s := NewWithAPIURL(nil, ts.URL)
	_, err := s.Scrape(context.Background(), model.ScraperInput{SearchTerm: "acme"})
	if err == nil {
		t.Fatal("expected error for 404 status, got nil")
	}
}
