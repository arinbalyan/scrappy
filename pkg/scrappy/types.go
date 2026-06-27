package scrappy

import (
	"sort"

	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/scraper"
)

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

// SiteInfo holds static metadata about a registered site.
type SiteInfo struct {
	Site        Site   `json:"site"`
	Method      string `json:"method"`   // html_parse | http_api | hybrid | playwright | rss
	NeedsAPIKey bool   `json:"needs_api_key"`
}

// AvailableSites returns all registered site names.
func (e *Engine) AvailableSites() []Site {
	sites := make([]Site, 0, len(e.scrapers))
	for s := range e.scrapers {
		sites = append(sites, Site(s))
	}
	return sites
}

// SiteInfo returns static metadata for all registered sites.
func (e *Engine) SiteInfo() []SiteInfo {
	info := make([]SiteInfo, 0, len(e.scrapers))
	for s := range e.scrapers {
		ms := model.Site(s)
		_, needsKey := requiredEnvVars[ms]
		info = append(info, SiteInfo{
			Site:        Site(s),
			Method:      scraper.Method(ms),
			NeedsAPIKey: needsKey,
		})
	}
	sort.Slice(info, func(i, j int) bool { return info[i].Site < info[j].Site })
	return info
}
