package naukri_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/arinbalyan/scrappy/internal/model"
	naukri "github.com/arinbalyan/scrappy/internal/scraper/naukri"
)

func TestNaukriScrape_ParsesSiteSpecificFields(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"jobDetails":[{"jobId":"123","title":"Software Engineer","companyName":"Acme","placeholders":["Bangalore"],"jobDescription":"Build APIs","jdURL":"https://www.naukri.com/job/123","tagsAndSkills":["Go","Microservices"],"experienceText":"3-5 Yrs","companyRating":"4.1","reviewCount":"210","vacancyCount":"4","wfhType":"Hybrid","footerPlaceholderLabel":"Remote"}]}`))
	}))
	defer srv.Close()

	s := naukri.NewWithAPIURL(srv.Client(), srv.URL)
	jobs, err := s.Scrape(context.Background(), model.ScraperInput{SearchTerm: "software engineer", ResultsWanted: 10})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job got %d", len(jobs))
	}
	j := jobs[0]
	if j.CompanyRating == nil || *j.CompanyRating != 4.1 {
		t.Fatalf("expected rating 4.1 got %+v", j.CompanyRating)
	}
	if j.CompanyReviews != 210 || j.VacancyCount != 4 || !j.IsRemote {
		t.Fatalf("unexpected parsed fields: %+v", j)
	}
	if len(j.Skills) != 2 || j.ExperienceRange != "3-5 Yrs" {
		t.Fatalf("unexpected skills/experience: %+v", j)
	}
}
