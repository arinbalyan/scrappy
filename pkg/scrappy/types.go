package scrappy

import "time"

// Site is a job-board / ATS identifier.
type Site string

// Country is a geographic search scope.
type Country string

// Location holds parsed geographic data.
type Location struct {
	City    string `json:"city,omitempty"`
	State   string `json:"state,omitempty"`
	Country string `json:"country,omitempty"`
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
	MinAmount *float64             `json:"min_amount"`
	MaxAmount *float64             `json:"max_amount"`
	Currency  string               `json:"currency,omitempty"`
}

// JobType matches the job-type enum.
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
	Verified bool   `json:"verified"`
	Source   string `json:"source,omitempty"`
	Role     bool   `json:"role,omitempty"`
}

// ScraperInput holds all parameters for a scraping run.
type ScraperInput struct {
	Sites               []Site          `json:"sites"`
	SearchTerm          string          `json:"search_term,omitempty"`
	Location            string          `json:"location,omitempty"`
	Country             Country         `json:"country,omitempty"`
	IsRemote            bool            `json:"is_remote"`
	JobType             JobType         `json:"job_type,omitempty"`
	EasyApply           bool            `json:"easy_apply"`
	ResultsWanted       int             `json:"results_wanted"`
	HoursOld            int             `json:"hours_old,omitempty"`
	SinceDate           string          `json:"since_date,omitempty"`
	DescriptionFormat   string          `json:"description_format,omitempty"`
	EnforceAnnualSalary bool            `json:"enforce_annual_salary"`
	EmailsOnly          bool            `json:"emails_only"`
	MinScore            int             `json:"min_score"`
	RemoteOnly          bool            `json:"remote_only"`
	VerifyEmail         bool            `json:"verify_email"`
	VerifyConcurrency   int             `json:"verify_concurrency,omitempty"`
	Proxy               string          `json:"-"`
	MemoryCapMB         int             `json:"memory_cap_mb,omitempty"`
	SearchTerms         []string        `json:"search_terms,omitempty"`
	Locations           []string        `json:"locations,omitempty"`
	SiteSearch          map[Site][]string `json:"site_search,omitempty"`
	SiteLocations       map[Site][]string `json:"site_locations,omitempty"`
	SiteResultsWanted   map[Site]int    `json:"site_results_wanted,omitempty"`
	SiteRPS             map[Site]int    `json:"site_rps,omitempty"`
	MaxRPS              int             `json:"max_rps,omitempty"`
	LogLevel            string          `json:"log_level,omitempty"`
}

// JobPost is the canonical scraped-job record returned to library consumers.
type JobPost struct {
	ID                  string         `json:"id,omitempty"`
	Title               string         `json:"title"`
	CompanyName         string         `json:"company_name,omitempty"`
	CompanyURL          string         `json:"company_url,omitempty"`
	JobURL              string         `json:"job_url"`
	JobURLDirect        string         `json:"job_url_direct,omitempty"`
	Location            Location       `json:"location,omitempty"`
	IsRemote            bool           `json:"is_remote,omitempty"`
	Description         string         `json:"description,omitempty"`
	JobType             string         `json:"job_type,omitempty"`
	DatePosted          *time.Time     `json:"date_posted,omitempty"`
	Site                Site           `json:"site"`
	FetchedAt           *time.Time     `json:"fetched_at,omitempty"`
	Emails              []Email        `json:"emails,omitempty"`
	Compensation        *Compensation  `json:"compensation"`
	Seniority           string         `json:"seniority,omitempty"`
	Department          string         `json:"department,omitempty"`
	Domain              string         `json:"domain,omitempty"`
	Industry            string         `json:"industry,omitempty"`
	CompanyLogoURL      string         `json:"company_logo_url,omitempty"`
	ApplyMethod         string         `json:"apply_method,omitempty"`
	JobLevel            string         `json:"job_level,omitempty"`
	CompanyIndustry     string         `json:"company_industry,omitempty"`
	CompanyDescription  string         `json:"company_description,omitempty"`
	Skills              []string       `json:"skills"`
	ExperienceRange     string         `json:"experience_range,omitempty"`
	QualityScore        int            `json:"quality_score,omitempty"`
}

// AvailableSites returns all registered site names.
func (e *Engine) AvailableSites() []Site {
	sites := make([]Site, 0, len(e.scrapers))
	for s := range e.scrapers {
		sites = append(sites, Site(s))
	}
	return sites
}
