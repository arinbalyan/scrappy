package indeed_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	indeedpkg "github.com/arinbalyan/scrappy/internal/scraper/indeed"
	"github.com/arinbalyan/scrappy/internal/model"
)

func TestIndeedScraper_ScrapeBasic(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"jobSearch": map[string]any{
					"pageInfo": map[string]any{"nextCursor": ""},
					"results": []any{map[string]any{
						"job": map[string]any{
							"key": "abc123",
							"title": "Software Engineer",
							"datePublished": float64(time.Now().UnixMilli()),
							"description": map[string]any{"html": "remote opportunity"},
							"location": map[string]any{
								"countryCode": "US",
								"admin1Code": "CA",
								"city": "San Francisco",
								"formatted": map[string]any{"long": "San Francisco, CA"},
							},
							"attributes": []any{map[string]any{"label": "Remote"}},
							"compensation": map[string]any{
								"currencyCode": "USD",
								"baseSalary": map[string]any{
									"unitOfWork": "YEAR",
									"range": map[string]any{"min": 120000.0, "max": 150000.0},
								},
							},
							"employer": map[string]any{
								"name": "Acme",
								"relativeCompanyPageUrl": "/cmp/acme",
								"dossier": map[string]any{
									"employerDetails": map[string]any{
										"addresses": []any{"San Francisco"},
										"industry": "software",
										"employeesLocalizedLabel": "1001-5000",
										"revenueLocalizedLabel": "$10M+",
										"briefDescription": "A software company",
									},
									"images": map[string]any{"squareLogoUrl": "https://img/logo.png"},
								},
							},
							"recruit": map[string]any{"viewJobUrl": "https://apply.example.com/abc"},
						},
					}},
				},
			},
		})
	}))
	defer server.Close()

	s := indeedpkg.NewWithAPIURL(server.Client(), server.URL)
	jobs, err := s.Scrape(context.Background(), model.ScraperInput{SearchTerm: "golang", ResultsWanted: 1})
	if err != nil {
		t.Fatalf("scrape: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
	if jobs[0].Title != "Software Engineer" {
		t.Fatalf("unexpected title: %s", jobs[0].Title)
	}
	if jobs[0].Compensation == nil || jobs[0].Compensation.MinAmount == nil {
		t.Fatalf("expected compensation in parsed job")
	}
}
