package model

import (
	"time"
)

// Site enumerates the supported job boards.
type Site string

const (
	SiteLinkedIn           Site = "linkedin"
	SiteIndeed             Site = "indeed"
	SiteZipRecruiter       Site = "zip_recruiter"
	SiteBayt               Site = "bayt"
	SiteBDJobs             Site = "bdjobs"
	SiteNaukri             Site = "naukri"
	SiteInternshala        Site = "internshala"
	SiteBuiltin            Site = "builtin"
	SiteStartupJobs        Site = "startupjobs"
	SiteGreenhouse         Site = "greenhouse"
	SiteGunIO              Site = "gunio"
	SiteHimalayas          Site = "himalayas"
	SiteHiringCafe         Site = "hiringcafe"
	SiteHuggingFaceJobs    Site = "huggingfacejobs"
	SiteJobindex           Site = "jobindex"
	SiteRemoteOK           Site = "remoteok"
	SiteRemotive           Site = "remotive"
	SiteRemoteFirstJobs    Site = "remotefirstjobs"
	SiteJobspresso         Site = "jobspresso"
	SiteHasJob             Site = "hasjob"
	SiteVueJobs            Site = "vuejobs"
	SiteLaraJobs           Site = "larajobs"
	SiteArbeitnow          Site = "arbeitnow"
	SiteHackerNews         Site = "hackernews"
	SiteCryptocurrencyJobs Site = "cryptocurrencyjobs"
	SiteAndroidJobs        Site = "androidjobs"
	SiteJobicy             Site = "jobicy"
	SiteDevOpsJobs         Site = "devopsjobs"
	SiteCrunchboard        Site = "crunchboard"
	SiteIOSDevJobs         Site = "iosdevjobs"
	SiteSwissDevJobs       Site = "swissdevjobs"
	SiteCryptoJobsList     Site = "cryptojobslist"
	SiteDevITJobs          Site = "devitjobs"
	SiteDribbble           Site = "dribbble"
	SiteAIJobs             Site = "aijobs"
	SiteWorkingNomads      Site = "workingnomads"
	SiteWuzzuf             Site = "wuzzuf"
	SiteYCJobs             Site = "ycjobs"
	SiteUKVisaJobs         Site = "ukvisajobs"
	SiteGoogle             Site = "google"
	SiteGlassdoor          Site = "glassdoor"
	SiteAdzuna             Site = "adzuna"
	SiteSimplyHired        Site = "simplyhired"
	SiteCareerBuilder      Site = "careerbuilder"
)

// AllSites returns every known site.
func AllSites() []Site {
	return []Site{
		SiteLinkedIn,
		SiteIndeed,
		SiteZipRecruiter,
		SiteBayt,
		SiteBDJobs,
		SiteNaukri,
		SiteInternshala,
		SiteBuiltin,
		SiteStartupJobs,
		SiteGreenhouse,
		SiteGunIO,
		SiteHimalayas,
		SiteHiringCafe,
		SiteHuggingFaceJobs,
		SiteJobindex,
		SiteRemoteOK,
		SiteRemotive,
		SiteRemoteFirstJobs,
		SiteJobspresso,
		SiteHasJob,
		SiteVueJobs,
		SiteLaraJobs,
		SiteArbeitnow,
		SiteHackerNews,
		SiteCryptocurrencyJobs,
		SiteAndroidJobs,
		SiteJobicy,
		SiteDevOpsJobs,
		SiteCrunchboard,
		SiteIOSDevJobs,
		SiteSwissDevJobs,
		SiteCryptoJobsList,
		SiteDevITJobs,
		SiteDribbble,
		SiteAIJobs,
		SiteWorkingNomads,
		SiteWuzzuf,
		SiteYCJobs,
		SiteUKVisaJobs,
		SiteGoogle,
		SiteGlassdoor,
		SiteAdzuna,
		SiteSimplyHired,
		SiteCareerBuilder,
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
