package model

import (
	"strings"
	"time"
)

// Site enumerates the supported job boards.
type Site string

const (
	SiteLinkedIn           Site = "linkedin"
	SiteIndeed             Site = "indeed"
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
	SiteCryptoJobsList     Site = "cryptojobslist"
	SiteDribbble           Site = "dribbble"
	SiteAIJobs             Site = "aijobs"
	SiteWorkingNomads      Site = "workingnomads"
	SiteYCJobs             Site = "ycjobs"
	SiteUKVisaJobs         Site = "ukvisajobs"
	SiteGoogle             Site = "google"
	SiteGlassdoor          Site = "glassdoor"
	SiteAdzuna             Site = "adzuna"
	SiteSimplyHired        Site = "simplyhired"
	SiteCareerBuilder      Site = "careerbuilder"
	SiteCareerjet          Site = "careerjet"
	SiteJooble             Site = "jooble"
	SiteDice               Site = "dice"
	SiteMonster            Site = "monster"
	SiteInfoJobs           Site = "infojobs"
	SiteReed               Site = "reed"
	SiteTheMuse            Site = "themuse"
	SiteJobsDB             Site = "jobsdb"
	SiteSnagajob           Site = "snagajob"
	SiteDjinni             Site = "djinni"
	SiteHeadHunter         Site = "headhunter"
	SiteMyCareersFuture    Site = "mycareersfuture"
	SiteJobStreet          Site = "jobstreet"
	Site4DayWeek           Site = "4dayweek"
	SiteEuroJobs           Site = "eurojobs"
	SiteFindwork           Site = "findwork"
	SiteArbeitsagentur    Site = "arbeitsagentur"
	SiteWeb3Career        Site = "web3career"
	SiteEchoJobs          Site = "echojobs"
	SiteNoDesk            Site = "nodesk"
	SiteJobTechDev        Site = "jobtechdev"
	SiteAuthenticJobs     Site = "authenticjobs"
	SiteEcoJobs           Site = "ecojobs"
	SiteGolangJobs        Site = "golangjobs"
	SiteLandingJobs       Site = "landingjobs"
	SiteRealWorkFromAnywhere Site = "realworkfromanywhere"

	// New sites being ported
	SiteBayt             Site = "bayt"
	SiteBDJobs           Site = "bdjobs"
	SiteBerlinStartupJobs Site = "berlinstartupjobs"
	SiteCanadaJobBank    Site = "canadajobbank"
	SiteCareerOneStop    Site = "careeronestop"
	SiteConservationJobs Site = "conservationjobs"
	SiteCoroflot         Site = "coroflot"
	SiteDevITJobs        Site = "devitjobs"
	SiteDrupalJobs       Site = "drupaljobs"
	SiteDuunitori        Site = "duunitori"
	SiteHabrCareer       Site = "habrcareer"
	SiteHigherEdJobs     Site = "higheredjobs"
	SiteIcrunchData      Site = "icrunchdata"
	SiteIOSDevJobs       Site = "iosdevjobs"
	SiteJobDataAPI       Site = "jobdataapi"
	SiteJobsAcUK         Site = "jobsacuk"
	SiteJobsCH           Site = "jobsch"
	SiteJobsInJapan      Site = "jobsinjapan"
	SiteJoinRise         Site = "joinrise"
	SiteNavJobs          Site = "navjobs"

	// Priority new site constants
	SiteElixirJobs       Site = "elixirjobs"
	SiteExa              Site = "exa"
	SiteFossJobs         Site = "fossjobs"
	SiteFranceTravail    Site = "francetravail"
	SiteFreelancerCom    Site = "freelancercom"
	SiteFunctionalWorks  Site = "functionalworks"
	SiteGermanTechJobs   Site = "germantechjobs"
	SiteGetOnBoard       Site = "getonboard"
	SiteGreenJobsBoard   Site = "greenjobsboard"
	SiteGuardianJobs     Site = "guardianjobs"
	SiteNoFluffJobs          Site = "nofluffjobs"
	SiteOpenSourceDesignJobs Site = "opensourcedesignjobs"
	SitePowerToFly           Site = "powertofly"
	SitePyJobs               Site = "pyjobs"
	SitePythonJobs           Site = "pythonjobs"
	SiteRailsJobs            Site = "railsjobs"
	SiteReliefWeb            Site = "reliefweb"
	SiteStepStone            Site = "stepstone"
	SiteSwissDevJobs         Site = "swissdevjobs"
	SiteTalroo               Site = "talroo"

	// Batch 2 — additional sources
	SiteTechCareers         Site = "techcareers"
	SiteTesla               Site = "tesla"
	SiteUNDPJobs            Site = "undpjobs"
	SiteUpwork              Site = "upwork"
	SiteUSAJobs             Site = "usajobs"
	SiteVirtualVocations    Site = "virtualvocations"
	SiteWellfound           Site = "wellfound"
	SiteWeWorkRemotely      Site = "weworkremotely"
	SiteWordPressJobs       Site = "wordpressjobs"
)

// AllSites returns every known site.
func AllSites() []Site {
	return []Site{
		SiteLinkedIn,
		SiteIndeed,
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
		SiteArbeitsagentur,
		SiteHackerNews,
		SiteCryptocurrencyJobs,
		SiteAndroidJobs,
		SiteJobicy,
		SiteDevOpsJobs,
		SiteCrunchboard,
		SiteCryptoJobsList,
		SiteDribbble,
		SiteAIJobs,
		SiteWorkingNomads,
		SiteYCJobs,
		SiteUKVisaJobs,
		SiteGoogle,
		SiteGlassdoor,
		SiteAdzuna,
		SiteSimplyHired,
		SiteCareerBuilder,
		SiteCareerjet,
		SiteJooble,
		SiteDice,
		SiteMonster,
		SiteInfoJobs,
		SiteReed,
		SiteTheMuse,
		SiteJobsDB,
		SiteSnagajob,
		SiteDjinni,
		SiteHeadHunter,
		SiteMyCareersFuture,
		SiteJobStreet,
		Site4DayWeek,
		SiteEuroJobs,
		SiteFindwork,
		SiteWeb3Career,
		SiteEchoJobs,
		SiteNoDesk,
		SiteJobTechDev,
		SiteAuthenticJobs,
		SiteEcoJobs,
		SiteGolangJobs,
		SiteLandingJobs,
		SiteRealWorkFromAnywhere,
		SiteBayt,
		SiteBDJobs,
		SiteBerlinStartupJobs,
		SiteCanadaJobBank,
		SiteCareerOneStop,
		SiteConservationJobs,
		SiteCoroflot,
		SiteDevITJobs,
		SiteDrupalJobs,
		SiteDuunitori,
		SiteHabrCareer,
		SiteHigherEdJobs,
		SiteIcrunchData,
		SiteIOSDevJobs,
		SiteJobDataAPI,
		SiteJobsAcUK,
		SiteJobsCH,
		SiteJobsInJapan,
		SiteJoinRise,
		SiteNavJobs,
		SiteElixirJobs,
		SiteExa,
		SiteFossJobs,
		SiteFranceTravail,
		SiteFreelancerCom,
		SiteFunctionalWorks,
		SiteGermanTechJobs,
		SiteGetOnBoard,
		SiteGreenJobsBoard,
		SiteGuardianJobs,
		SiteNoFluffJobs,
		SiteOpenSourceDesignJobs,
		SitePowerToFly,
		SitePyJobs,
		SitePythonJobs,
		SiteRailsJobs,
		SiteReliefWeb,
		SiteStepStone,
		SiteSwissDevJobs,
		SiteTalroo,
		SiteTechCareers,
		SiteTesla,
		SiteUNDPJobs,
		SiteUpwork,
		SiteUSAJobs,
		SiteVirtualVocations,
		SiteWellfound,
		SiteWeWorkRemotely,
		SiteWordPressJobs,
	}
}

// Country enumerates supported search countries.
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
	parts := make([]string, 0, 3)
	if l.City != "" {
		parts = append(parts, l.City)
	}
	if l.State != "" {
		parts = append(parts, l.State)
	}
	if l.Country != "" {
		parts = append(parts, l.Country)
	}
	return strings.Join(parts, ", ")
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
	Verified bool   `json:"verified"`       // MX lookup passed
	Source   string `json:"source"`         // description | company_page | mailto | direct
	Role     bool   `json:"role,omitempty"` // info@, admin@, support@, etc.
}

// JobPost is the canonical scraped-job record.
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
	Site         string     `json:"site"`
	FetchedAt    *time.Time `json:"fetched_at,omitempty"`

	Emails         []Email       `json:"emails,omitempty"`
	Compensation   *Compensation `json:"compensation"`
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
	Skills          []string `json:"skills"`
	ExperienceRange string   `json:"experience_range,omitempty"`
	CompanyRating   *float64 `json:"company_rating,omitempty"`
	CompanyReviews  int      `json:"company_reviews_count,omitempty"`
	VacancyCount    int      `json:"vacancy_count,omitempty"`
	WorkFromHome    string   `json:"work_from_home_type,omitempty"`

	// Computed fields (set by internal/quality)
	QualityScore int `json:"quality_score,omitempty"`
}

// JobResponse wraps a slice of JobPost.
type JobResponse struct {
	Jobs []JobPost `json:"jobs"`
}
