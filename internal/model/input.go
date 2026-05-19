package model

// ScraperInput holds all parameters for a scraping run (mirrors JobSpy's ScraperInput).
type ScraperInput struct {
	Sites                 []Site   `json:"sites"`
	SearchTerm            string   `json:"search_term,omitempty"`
	GoogleSearchTerm      string   `json:"google_search_term,omitempty"`
	Location              string   `json:"location,omitempty"`
	Country               Country  `json:"country,omitempty"`
	DistanceMiles         int      `json:"distance,omitempty"`
	IsRemote              bool     `json:"is_remote"`
	JobType               JobType  `json:"job_type,omitempty"`
	EasyApply             bool     `json:"easy_apply"`
	ResultsWanted         int      `json:"results_wanted"`
	Offset                int      `json:"offset"`
	HoursOld              int      `json:"hours_old,omitempty"`
	DescriptionFormat     string   `json:"description_format,omitempty"` // markdown | html | plain
	EnforceAnnualSalary   bool     `json:"enforce_annual_salary"`
	LinkedInFetchDesc     bool     `json:"linkedin_fetch_description"`
	LinkedInCompanyIDs    []int    `json:"linkedin_company_ids,omitempty"`

	// scrappy-specific
	EmailsOnly            bool `json:"emails_only"`
	Dedup                 bool `json:"dedup"`
	DedupByCompany        bool `json:"dedup_by_company"`
	MinScore              int  `json:"min_score"`
	RemoteOnly            bool `json:"remote_only"`
	VerifyEmail           bool `json:"verify_email"`
	VerifyConcurrency     int  `json:"verify_concurrency"`
	ExcludeRoles          bool `json:"exclude_roles"`
	EmailMaxPerJob        int  `json:"email_max_per_job"`
	EmailEnrich           bool `json:"email_enrich"`
	EmailEnrichDomains    string `json:"email_enrich_domains,omitempty"`
	LinkedInStrategy      string `json:"linkedin_strategy,omitempty"` // "" | "rotate"
	CSVEmailsOnly         bool `json:"csv_emails_only"`
	MaxRPS                int  `json:"max_rps"`
	SiteRPS               map[Site]int `json:"site_rps,omitempty"`

	// proxies and resilience
	Proxy                 string `json:"-"` // not serialised; consumed by transport layer
	LocalProxyPort        int    `json:"-"`
	ProxyHealthCheck      bool   `json:"proxy_health_check"`
	Retries               int    `json:"retries"`
}
