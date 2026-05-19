package scrappy

import (
	"fmt"
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
			if input.HoursOld > 0 { count++ }
			if input.EasyApply { count++ }
			if input.JobType != "" || input.IsRemote { count++ }
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
		}
	}
	return r
}
