package scrappy

import (
	"fmt"

	"github.com/arinbalyan/scrappy/internal/model"
)

type ConstraintResult struct {
	Warnings []string
	Errors   []string
}

// EvaluateConstraints checks the input for site-specific constraints and limitations.
// Accepts public types — for external consumers like JobHunter.
func EvaluateConstraints(input ScraperInput) ConstraintResult {
	return evaluateConstraints(scraperInputToModel(input))
}

// EvaluateConstraintsInternal accepts internal types (used by cmd/scrappy).
func EvaluateConstraintsInternal(input model.ScraperInput) ConstraintResult {
	return evaluateConstraints(input)
}

func evaluateConstraints(input model.ScraperInput) ConstraintResult {
	hoursOldSupported := map[model.Site]bool{model.SiteIndeed: true, model.SiteLinkedIn: true}
	r := ConstraintResult{}
	for _, s := range input.Sites {
		if input.HoursOld > 0 && !hoursOldSupported[s] {
			r.Warnings = append(r.Warnings, fmt.Sprintf("hours_old is ignored for site=%s (supported: indeed, linkedin)", s))
		}
		switch s {
		case model.SiteIndeed:
			count := 0
			if input.HoursOld > 0 {
				count++
			}
			if input.JobType != "" || input.IsRemote {
				count++
			}
			if count > 1 {
				r.Warnings = append(r.Warnings, "Indeed supports only one of: hours_old OR job_type/is_remote")
			}
		case model.SiteLinkedIn:
		}
	}
	return r
}
