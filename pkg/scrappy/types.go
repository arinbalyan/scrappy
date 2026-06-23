package scrappy

import "github.com/arinbalyan/scrappy/internal/model"

// Type aliases — external consumers import pkg/scrappy, internal code uses model.
// Ponytail: no duplication, no conversion functions, one source of truth.

type Site = model.Site
type Country = model.Country
type Location = model.Location
type CompensationInterval = model.CompensationInterval
type Compensation = model.Compensation
type JobType = model.JobType
type Email = model.Email
type ScraperInput = model.ScraperInput
type JobPost = model.JobPost

// Re-export constants so consumers don't need to import model.
const (
	IntervalYearly  = model.IntervalYearly
	IntervalMonthly = model.IntervalMonthly
	IntervalWeekly  = model.IntervalWeekly
	IntervalDaily   = model.IntervalDaily
	IntervalHourly  = model.IntervalHourly

	JobTypeFullTime   = model.JobTypeFullTime
	JobTypePartTime   = model.JobTypePartTime
	JobTypeContract   = model.JobTypeContract
	JobTypeInternship = model.JobTypeInternship
	JobTypeTemporary  = model.JobTypeTemporary
)

// AvailableSites returns all registered site names.
func (e *Engine) AvailableSites() []Site {
	sites := make([]Site, 0, len(e.scrapers))
	for s := range e.scrapers {
		sites = append(sites, Site(s))
	}
	return sites
}
