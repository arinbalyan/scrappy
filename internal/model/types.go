package model

import (
	"time"
)

// Site enumerates the supported job boards.
type Site string

const (
	SiteLinkedIn      Site = "linkedin"
	SiteIndeed        Site = "indeed"
	SiteZipRecruiter  Site = "zip_recruiter"
	SiteGlassdoor     Site = "glassdoor"
	SiteGoogle        Site = "google"
	SiteBayt          Site = "bayt"
	SiteNaukri        Site = "naukri"
	SiteBDJobs        Site = "bdjobs"
	SiteWellfound       Site = "wellfound"
	SiteHimalayas       Site = "himalayas"
	SiteWeWorkRemotely  Site = "weworkremotely"
	SiteRemoteCo        Site = "remoteco"
	SiteRemoteOK        Site = "remoteok"
	SiteRemotive        Site = "remotive"
	SiteBuiltIn         Site = "builtin"
	SiteContra          Site = "contra"
	SiteToptal          Site = "toptal"
	SiteGunIO           Site = "gunio"
	SiteBraintrust      Site = "braintrust"
	SiteLemonIO         Site = "lemonio"
	SiteYCJobs          Site = "ycjobs"
	SitePallet          Site = "pallet"
	SiteGetro           Site = "getro"
	SiteMLJobs          Site = "mljobs"
	SiteAIJobs          Site = "aijobs"
	SiteHuggingFaceJobs Site = "huggingfacejobs"
	SiteOtta          Site = "otta"
	SiteLever         Site = "lever"
	SiteGreenhouse    Site = "greenhouse"
	SiteWorkableJobs  Site = "workable_jobs"
	SiteMyWorkdayJobs Site = "myworkdayjobs"
	SiteAdzuna        Site = "adzuna"
	SiteSeek          Site = "seek"
	SiteWorkingNomads Site = "workingnomads"
	SiteStartupJobs   Site = "startupjobs"
	SiteGradcracker   Site = "gradcracker"
	SiteHiringCafe    Site = "hiringcafe"
	SiteJobindex      Site = "jobindex"
	SiteUKVisaJobs    Site = "ukvisajobs"
	SiteWuzzuf        Site = "wuzzuf"
	SiteRemotiveAPI   Site = "remotive_api"
)

// AllSites returns every known site.
func AllSites() []Site {
	return []Site{
		SiteLinkedIn, SiteIndeed, SiteZipRecruiter,
		SiteGoogle, SiteBayt, SiteBDJobs,
		SiteWellfound, SiteHimalayas, SiteWeWorkRemotely, SiteRemoteCo, SiteRemoteOK, SiteRemotive, SiteRemotiveAPI, SiteBuiltIn,
		SiteContra, SiteToptal, SiteGunIO, SiteBraintrust, SiteLemonIO,
		SiteYCJobs, SitePallet, SiteGetro,
		SiteMLJobs, SiteAIJobs, SiteHuggingFaceJobs,
		SiteOtta, SiteLever, SiteGreenhouse, SiteWorkableJobs, SiteMyWorkdayJobs,
		SiteAdzuna, SiteWorkingNomads, SiteStartupJobs,
		SiteJobindex, SiteUKVisaJobs, SiteWuzzuf,
	}
}

// Country enumerates supported search countries (mirrors JobSpy).
type Country string

const (
	CountryUSA       Country = "usa"
	CountryCanada    Country = "canada"
	CountryUK        Country = "uk"
	CountryGermany   Country = "germany"
	CountryFrance    Country = "france"
	CountryIndia     Country = "india"
	CountryAustralia Country = "australia"
	// ... extend as needed
)

// Location holds parsed geographic data.
type Location struct {
	City    string `json:"city,omitempty"`
	State   string `json:"state,omitempty"`
	Country string `json:"country,omitempty"`
}

func (l Location) Display() string {
	parts := []string{}
	if l.City != "" {
		parts = append(parts, l.City)
	}
	if l.State != "" {
		parts = append(parts, l.State)
	}
	if l.Country != "" {
		parts = append(parts, l.Country)
	}
	return join(parts, ", ")
}

func join(parts []string, sep string) string {
	switch len(parts) {
	case 0:
		return ""
	case 1:
		return parts[0]
	case 2:
		return parts[0] + sep + parts[1]
	default:
		return parts[0] + sep + parts[1] + sep + parts[2]
	}
}

// CompensationInterval is the pay period.
type CompensationInterval string

const (
	IntervalYearly  CompensationInterval = "yearly"
	IntervalMonthly CompensationInterval = "monthly"
	IntervalWeekly  CompensationInterval = "weekly"
	IntervalDaily   CompensationInterval = "daily"
	IntervalHourly  CompensationInterval = "hourly"
)

// Compensation holds salary data.
type Compensation struct {
	Interval  CompensationInterval `json:"interval,omitempty"`
	MinAmount *float64             `json:"min_amount,omitempty"`
	MaxAmount *float64             `json:"max_amount,omitempty"`
	Currency  string               `json:"currency,omitempty"` // default USD
}

// JobType mirrors JobSpy's enum.
type JobType string

const (
	JobTypeFullTime   JobType = "fulltime"
	JobTypePartTime   JobType = "parttime"
	JobTypeContract   JobType = "contract"
	JobTypeInternship JobType = "internship"
	JobTypeTemporary  JobType = "temporary"
)

// Email is a single extracted and optionally verified address.
type Email struct {
	Addr     string `json:"addr"`
	Verified bool   `json:"verified"`       // MX lookup passed
	Source   string `json:"source"`         // description | company_page | mailto | direct
	Role     bool   `json:"role,omitempty"` // info@, admin@, support@, etc.
}

// JobPost is the canonical scraped-job record, extending JobSpy's model.
type JobPost struct {
	ID           string     `json:"id,omitempty"`
	Title        string     `json:"title"`
	CompanyName  string     `json:"company_name,omitempty"`
	CompanyURL   string     `json:"company_url,omitempty"`
	JobURL       string     `json:"job_url"`
	JobURLDirect string     `json:"job_url_direct,omitempty"`
	Location     Location   `json:"location,omitempty"`
	IsRemote     bool       `json:"is_remote,omitempty"`
	Description  string     `json:"description,omitempty"`
	JobType      string     `json:"job_type,omitempty"`
	DatePosted   *time.Time `json:"date_posted,omitempty"`

	Emails         []Email       `json:"emails,omitempty"`
	Compensation   *Compensation `json:"compensation,omitempty"`
	Seniority      string        `json:"seniority,omitempty"`  // entry | mid | senior | lead
	Department     string        `json:"department,omitempty"` // eng | data | product | ...
	Domain         string        `json:"domain,omitempty"`     // company domain from email / URL
	Industry       string        `json:"industry,omitempty"`
	CompanyLogoURL string        `json:"company_logo_url,omitempty"`
	ApplyMethod    string        `json:"apply_method,omitempty"` // easy_apply | email | external_url

	// LinkedIn-specific
	JobLevel string `json:"job_level,omitempty"`

	// LinkedIn + Indeed
	CompanyIndustry string `json:"company_industry,omitempty"`

	// Indeed-specific
	CompanyAddresses    string `json:"company_addresses,omitempty"`
	CompanyNumEmployees string `json:"company_num_employees,omitempty"`
	CompanyRevenue      string `json:"company_revenue,omitempty"`
	CompanyDescription  string `json:"company_description,omitempty"`
	CompanyLogo         string `json:"company_logo,omitempty"`

	// Naukri-specific
	Skills          []string `json:"skills,omitempty"`
	ExperienceRange string   `json:"experience_range,omitempty"`
	CompanyRating   *float64 `json:"company_rating,omitempty"`
	CompanyReviews  int      `json:"company_reviews_count,omitempty"`
	VacancyCount    int      `json:"vacancy_count,omitempty"`
	WorkFromHome    string   `json:"work_from_home_type,omitempty"`

	// Computed fields (set by internal/quality)
	QualityScore int `json:"quality_score,omitempty"`
}

// JobResponse wraps a slice of JobPost (mirrors JobSpy model).
type JobResponse struct {
	Jobs []JobPost `json:"jobs"`
}
