package types

import "time"

type JobPosting struct {
	ID, Title, Company, Location, Description, URL, Source string
	PostedAt      *time.Time
	JobType, Salary, SalaryPeriod, Currency, Industry string
	SalaryMin, SalaryMax *float64
	EasyApply, Remote *bool
	CompanyURL string
}

func (j JobPosting) DedupKey() string { return j.URL }
