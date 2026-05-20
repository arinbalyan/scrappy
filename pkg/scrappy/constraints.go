package scrappy

import (
	"fmt"
	"strings"

	"github.com/arinbalyan/scrappy/internal/model"
)

type ConstraintResult struct {
	Warnings []string
	Errors   []string
}

func EvaluateConstraints(input model.ScraperInput) ConstraintResult {
	r := ConstraintResult{}
	for _, s := range input.Sites {
		switch s {
		case model.SiteIndeed:
			count := 0
			if input.HoursOld > 0 {
				count++
			}
			if input.EasyApply {
				count++
			}
			if input.JobType != "" || input.IsRemote {
				count++
			}
			if count > 1 {
				r.Warnings = append(r.Warnings, "Indeed supports only one of: hours_old OR easy_apply OR job_type/is_remote")
			}
		case model.SiteLinkedIn:
			if input.HoursOld > 0 && input.EasyApply {
				r.Warnings = append(r.Warnings, "LinkedIn cannot reliably combine hours_old with easy_apply")
			}
			if input.LinkedInFetchDesc {
				r.Warnings = append(r.Warnings, "linkedin_fetch_description adds O(n) requests and can trigger rate limits")
			}
		case model.SiteGoogle:
			if input.GoogleSearchTerm == "" {
				r.Warnings = append(r.Warnings, "Google scraper works best with --google-search-term (free-text query)")
			}
		case model.SiteZipRecruiter:
			if input.Country != "" {
				r.Warnings = append(r.Warnings, fmt.Sprintf("ZipRecruiter ignores country=%q and primarily uses location", input.Country))
			}
		case model.SiteWorkableJobs:
			if len(input.WorkableSeeds) == 0 {
				r.Warnings = append(r.Warnings, "workable_jobs works best with --workable-seeds or SCRAPPY_WORKABLE_SEEDS")
			}
		case model.SiteMyWorkdayJobs:
			if len(input.WorkdaySeeds) == 0 {
				r.Warnings = append(r.Warnings, "myworkdayjobs works best with --workday-seeds or SCRAPPY_WORKDAY_SEEDS")
			}
		case model.SiteAdzuna:
			if strings.TrimSpace(input.AdzunaAppID) == "" || strings.TrimSpace(input.AdzunaAppKey) == "" {
				r.Warnings = append(r.Warnings, "adzuna requires --adzuna-app-id/--adzuna-app-key or SCRAPPY_ADZUNA_APP_ID/SCRAPPY_ADZUNA_APP_KEY")
			}
		}
	}
	return r
}
