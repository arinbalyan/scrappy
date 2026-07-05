package scrappy

import (
	"context"
	"fmt"
	"net"
	netmail "net/mail"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"runtime/metrics"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/BurntSushi/toml"

	internalemail "github.com/arinbalyan/scrappy/internal/email"
	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/normalize"
	"github.com/arinbalyan/scrappy/internal/quality"
	"github.com/arinbalyan/scrappy/internal/scraper"
	aijobsscraper "github.com/arinbalyan/scrappy/internal/scraper/aijobs"
	androidjobsscraper "github.com/arinbalyan/scrappy/internal/scraper/androidjobs"
	arbeitnowscraper "github.com/arinbalyan/scrappy/internal/scraper/arbeitnow"
	builtinscraper "github.com/arinbalyan/scrappy/internal/scraper/builtin"
	crunchboardscraper "github.com/arinbalyan/scrappy/internal/scraper/crunchboard"
	cryptocurrencyjobsscraper "github.com/arinbalyan/scrappy/internal/scraper/cryptocurrencyjobs"
	cryptojobslistscraper "github.com/arinbalyan/scrappy/internal/scraper/cryptojobslist"

	devopsjobsscraper "github.com/arinbalyan/scrappy/internal/scraper/devopsjobs"
	googlescraper "github.com/arinbalyan/scrappy/internal/scraper/google"
	greenhousescraper "github.com/arinbalyan/scrappy/internal/scraper/greenhouse"
	gunioscraper "github.com/arinbalyan/scrappy/internal/scraper/gunio"
	hackernewsscraper "github.com/arinbalyan/scrappy/internal/scraper/hackernews"
	hasjobscraper "github.com/arinbalyan/scrappy/internal/scraper/hasjob"
	himalayasscraper "github.com/arinbalyan/scrappy/internal/scraper/himalayas"
	hiringcafescraper "github.com/arinbalyan/scrappy/internal/scraper/hiringcafe"
	huggingfacejobsscraper "github.com/arinbalyan/scrappy/internal/scraper/huggingfacejobs"
	indeedscraper "github.com/arinbalyan/scrappy/internal/scraper/indeed"
	internshalascraper "github.com/arinbalyan/scrappy/internal/scraper/internshala"
	jobicyscraper "github.com/arinbalyan/scrappy/internal/scraper/jobicy"
	jobindexscraper "github.com/arinbalyan/scrappy/internal/scraper/jobindex"
	jobspressoscraper "github.com/arinbalyan/scrappy/internal/scraper/jobspresso"
	jobstreetscraper "github.com/arinbalyan/scrappy/internal/scraper/jobstreet"

	linkedinscraper "github.com/arinbalyan/scrappy/internal/scraper/linkedin"
	reedscraper "github.com/arinbalyan/scrappy/internal/scraper/reed"
	remotefirstjobsscraper "github.com/arinbalyan/scrappy/internal/scraper/remotefirstjobs"
	remoteokscraper "github.com/arinbalyan/scrappy/internal/scraper/remoteok"
	remotivescraper "github.com/arinbalyan/scrappy/internal/scraper/remotive"
	startupjobsscraper "github.com/arinbalyan/scrappy/internal/scraper/startupjobs"

	ukvisajobsscraper "github.com/arinbalyan/scrappy/internal/scraper/ukvisajobs"

	fwdayweekscraper "github.com/arinbalyan/scrappy/internal/scraper/4dayweek"
	adzunascraper "github.com/arinbalyan/scrappy/internal/scraper/adzuna"
	careerbuilderscraper "github.com/arinbalyan/scrappy/internal/scraper/careerbuilder"
	careerjetscraper "github.com/arinbalyan/scrappy/internal/scraper/careerjet"
	dicescraper "github.com/arinbalyan/scrappy/internal/scraper/dice"
	djinniscraper "github.com/arinbalyan/scrappy/internal/scraper/djinni"
	echojobsscraper "github.com/arinbalyan/scrappy/internal/scraper/echojobs"
	eurojobsscraper "github.com/arinbalyan/scrappy/internal/scraper/eurojobs"
	findworkscraper "github.com/arinbalyan/scrappy/internal/scraper/findwork"
	headhunterscraper "github.com/arinbalyan/scrappy/internal/scraper/headhunter"
	infojobsscraper "github.com/arinbalyan/scrappy/internal/scraper/infojobs"
	jobsscraper "github.com/arinbalyan/scrappy/internal/scraper/jobsdb"
	jobtechdevscraper "github.com/arinbalyan/scrappy/internal/scraper/jobtechdev"
	monsterscraper "github.com/arinbalyan/scrappy/internal/scraper/monster"
	mycareersfuturescraper "github.com/arinbalyan/scrappy/internal/scraper/mycareersfuture"
	simplyhiredscraper "github.com/arinbalyan/scrappy/internal/scraper/simplyhired"
	snagajobscraper "github.com/arinbalyan/scrappy/internal/scraper/snagajob"
	themusescraper "github.com/arinbalyan/scrappy/internal/scraper/themuse"
	web3careerscraper "github.com/arinbalyan/scrappy/internal/scraper/web3career"
	workingnomadsscraper "github.com/arinbalyan/scrappy/internal/scraper/workingnomads"
	ycjobsscraper "github.com/arinbalyan/scrappy/internal/scraper/ycjobs"

	arbeitsagenturscraper "github.com/arinbalyan/scrappy/internal/scraper/arbeitsagentur"
	authenticjobsscraper "github.com/arinbalyan/scrappy/internal/scraper/authenticjobs"
	baytscraper "github.com/arinbalyan/scrappy/internal/scraper/bayt"
	berlinstartupjobsscraper "github.com/arinbalyan/scrappy/internal/scraper/berlinstartupjobs"
	canadajobbankscraper "github.com/arinbalyan/scrappy/internal/scraper/canadajobbank"
	careeronestopscraper "github.com/arinbalyan/scrappy/internal/scraper/careeronestop"
	conservationjobsscraper "github.com/arinbalyan/scrappy/internal/scraper/conservationjobs"
	coroflotscraper "github.com/arinbalyan/scrappy/internal/scraper/coroflot"
	ecojobsscraper "github.com/arinbalyan/scrappy/internal/scraper/ecojobs"
	golangjobsscraper "github.com/arinbalyan/scrappy/internal/scraper/golangjobs"
	landingjobsscraper "github.com/arinbalyan/scrappy/internal/scraper/landingjobs"
	realworkfromanywherescraper "github.com/arinbalyan/scrappy/internal/scraper/realworkfromanywhere"

	drupaljobsscraper "github.com/arinbalyan/scrappy/internal/scraper/drupaljobs"

	exascraper "github.com/arinbalyan/scrappy/internal/scraper/exa"
	fossjobsscraper "github.com/arinbalyan/scrappy/internal/scraper/fossjobs"
	francetravailscraper "github.com/arinbalyan/scrappy/internal/scraper/francetravail"
	freelancercomscraper "github.com/arinbalyan/scrappy/internal/scraper/freelancercom"
	functionalworkscraper "github.com/arinbalyan/scrappy/internal/scraper/functionalworks"

	getonboardscraper "github.com/arinbalyan/scrappy/internal/scraper/getonboard"

	higheredjobsscraper "github.com/arinbalyan/scrappy/internal/scraper/higheredjobs"
	icrunchdatascraper "github.com/arinbalyan/scrappy/internal/scraper/icrunchdata"

	jobdataapiscraper "github.com/arinbalyan/scrappy/internal/scraper/jobdataapi"

	jobschscraper "github.com/arinbalyan/scrappy/internal/scraper/jobsch"
	jobsinjapanscraper "github.com/arinbalyan/scrappy/internal/scraper/jobsinjapan"
	joinrisescraper "github.com/arinbalyan/scrappy/internal/scraper/joinrise"

	nofluffjobsscraper "github.com/arinbalyan/scrappy/internal/scraper/nofluffjobs"

	pythonjobsscraper "github.com/arinbalyan/scrappy/internal/scraper/pythonjobs"
	railsjobsscraper "github.com/arinbalyan/scrappy/internal/scraper/railsjobs"
	stepstonescraper "github.com/arinbalyan/scrappy/internal/scraper/stepstone"

	talrooscraper "github.com/arinbalyan/scrappy/internal/scraper/talroo"

	teslascraper "github.com/arinbalyan/scrappy/internal/scraper/tesla"

	upworkscraper "github.com/arinbalyan/scrappy/internal/scraper/upwork"
	usajobsscraper "github.com/arinbalyan/scrappy/internal/scraper/usajobs"

	academiccareersscraper "github.com/arinbalyan/scrappy/internal/scraper/academiccareers"
	wellfoundscraper "github.com/arinbalyan/scrappy/internal/scraper/wellfound"
	weworkremotelyscraper "github.com/arinbalyan/scrappy/internal/scraper/weworkremotely"
	wordpressjobsscraper "github.com/arinbalyan/scrappy/internal/scraper/wordpressjobs"
	wuzzufscraper "github.com/arinbalyan/scrappy/internal/scraper/wuzzuf"
	ziprecruiterscraper "github.com/arinbalyan/scrappy/internal/scraper/ziprecruiter"

	// ATS scrapers (non-prefixed directory names)
	adpscraper "github.com/arinbalyan/scrappy/internal/scraper/adp"
	ashbyscraper "github.com/arinbalyan/scrappy/internal/scraper/ashby"
	avaturescraper "github.com/arinbalyan/scrappy/internal/scraper/avature"
	bamboohrscraper "github.com/arinbalyan/scrappy/internal/scraper/bamboohr"
	breezyhrscraper "github.com/arinbalyan/scrappy/internal/scraper/breezyhr"
	bullhornscraper "github.com/arinbalyan/scrappy/internal/scraper/bullhorn"
	comeetscraper "github.com/arinbalyan/scrappy/internal/scraper/comeet"
	crelatescraper "github.com/arinbalyan/scrappy/internal/scraper/crelate"
	deelscraper "github.com/arinbalyan/scrappy/internal/scraper/deel"
	fountainscraper "github.com/arinbalyan/scrappy/internal/scraper/fountain"
	freshteamscraper "github.com/arinbalyan/scrappy/internal/scraper/freshteam"
	gemscraper "github.com/arinbalyan/scrappy/internal/scraper/gem"
	hiringthingscraper "github.com/arinbalyan/scrappy/internal/scraper/hiringthing"
	icimsscraper "github.com/arinbalyan/scrappy/internal/scraper/icims"
	ismartrecruitscraper "github.com/arinbalyan/scrappy/internal/scraper/ismartrecruit"
	jazzhrscraper "github.com/arinbalyan/scrappy/internal/scraper/jazzhr"
	jobscorescraper "github.com/arinbalyan/scrappy/internal/scraper/jobscore"
	jobvitescraper "github.com/arinbalyan/scrappy/internal/scraper/jobvite"
	jobylonscraper "github.com/arinbalyan/scrappy/internal/scraper/jobylon"
	joincomscraper "github.com/arinbalyan/scrappy/internal/scraper/joincom"
	loxoscraper "github.com/arinbalyan/scrappy/internal/scraper/loxo"
	manatalscraper "github.com/arinbalyan/scrappy/internal/scraper/manatal"

	// ATS scrapers (ats- prefixed directories)
	atsmercorscraper "github.com/arinbalyan/scrappy/internal/scraper/ats-mercor"
	atsoraclescraper "github.com/arinbalyan/scrappy/internal/scraper/ats-oracle"
	atspersonioscraper "github.com/arinbalyan/scrappy/internal/scraper/ats-personio"
	atsphenomscraper "github.com/arinbalyan/scrappy/internal/scraper/ats-phenom"
	atspinpointscraper "github.com/arinbalyan/scrappy/internal/scraper/ats-pinpoint"
	atsrecruiteescraper "github.com/arinbalyan/scrappy/internal/scraper/ats-recruitee"
	atsrecruiterflowscraper "github.com/arinbalyan/scrappy/internal/scraper/ats-recruiterflow"
	atsripplingscraper "github.com/arinbalyan/scrappy/internal/scraper/ats-rippling"
	atssmartrecruiterscraper "github.com/arinbalyan/scrappy/internal/scraper/ats-smartrecruiters"
	atssuccessfactorscraper "github.com/arinbalyan/scrappy/internal/scraper/ats-successfactors"
	atstalentlyftscraper "github.com/arinbalyan/scrappy/internal/scraper/ats-talentlyft"
	atstaleoscraper "github.com/arinbalyan/scrappy/internal/scraper/ats-taleo"
	atsteamtailorscraper "github.com/arinbalyan/scrappy/internal/scraper/ats-teamtailor"
	atstrakstarscraper "github.com/arinbalyan/scrappy/internal/scraper/ats-trakstar"
	atsukgscraper "github.com/arinbalyan/scrappy/internal/scraper/ats-ukg"
	atsworkablescraper "github.com/arinbalyan/scrappy/internal/scraper/ats-workable"
	atsworkdayscraper "github.com/arinbalyan/scrappy/internal/scraper/ats-workday"

	"github.com/arinbalyan/scrappy/internal/util"
)

// requiredEnvVars maps sites to environment variables that must be set
// before the scraper can function.  The engine skips these sites with
// a clear WARN message instead of wasting time on a doomed request.
var requiredEnvVars = map[model.Site][]string{
	model.SiteAdzuna:         {"ADZUNA_APP_ID", "ADZUNA_APP_KEY"},
	model.SiteCareerjet:      {"CAREERJET_AFFID"},
	model.SiteInfoJobs:       {"INFOJOBS_CLIENT_ID", "INFOJOBS_CLIENT_SECRET"},
	model.SiteFindwork:       {"FINDWORK_API_KEY"},
	model.SiteArbeitsagentur: {"ARBEITSAGENTUR_API_KEY"},
	model.SiteWeb3Career:     {"WEB3CAREER_API_TOKEN"},
	model.SiteJobTechDev:     {"JOBTECHDEV_API_KEY"},
	model.SiteAuthenticJobs:  {"AUTHENTICJOBS_API_KEY"},
	model.SiteFranceTravail:  {"FRANCETRAVAIL_CLIENT_ID", "FRANCETRAVAIL_CLIENT_SECRET"},
	model.SiteTalroo:         {"TALROO_PUBLISHER_ID", "TALROO_PUBLISHER_PASS"},
}

// Option configures Engine construction.
type Option func(*Engine)

// WithConfig loads scrappy's config.toml and populates per-site search terms,
// locations, and country settings on every Scrape() call. The file is loaded
// once at engine creation; missing file is silently ignored.
func WithConfig(path string) Option {
	return func(e *Engine) {
		if path == "" {
			return
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return // silently skip missing config
		}
		var cfg struct {
			Defaults struct {
				Search   []string `toml:"search"`
				Location []string `toml:"location"`
			} `toml:"defaults"`
			Sites map[string]struct {
				Search   []string `toml:"search"`
				Location []string `toml:"location"`
				Country  string   `toml:"country"`
			} `toml:"sites"`
		}
		if err := toml.Unmarshal(data, &cfg); err != nil {
			return
		}
		e.configSearch = cfg.Defaults.Search
		e.configLocation = cfg.Defaults.Location
		e.configSites = cfg.Sites
		e.configPath = path
	}
}

type Engine struct {
	scrapers     map[model.Site]scraper.Scraper
	telemetry    RunTelemetry
	siteFailOpen bool

	configSearch  []string
	configLocation []string
	configSites   map[string]struct {
		Search   []string `toml:"search"`
		Location []string `toml:"location"`
		Country  string   `toml:"country"`
	}
	configPath string // saved by WithConfig for ReloadConfig

	// playwrightCached caches whether Node.js + playwright is available.
	// Checked lazily on first required playwright scrape.
	playwrightCached     *bool
	playwrightCachedOnce sync.Once
}

func NewEngine(opts ...Option) *Engine {
	s := []scraper.Scraper{
		indeedscraper.New(nil),
		linkedinscraper.New(nil),
		internshalascraper.New(nil),
		builtinscraper.New(nil),
		startupjobsscraper.New(nil),
		greenhousescraper.New(nil),
		gunioscraper.New(nil),
		himalayasscraper.New(nil),
		hiringcafescraper.New(nil),
		huggingfacejobsscraper.New(nil),
		jobindexscraper.New(nil),
		reedscraper.New(nil),
		remoteokscraper.New(nil),
		remotivescraper.New(nil),
		remotefirstjobsscraper.New(nil),
		jobspressoscraper.New(nil),
		hasjobscraper.New(nil),
		arbeitnowscraper.New(nil),
		hackernewsscraper.New(nil),
		cryptocurrencyjobsscraper.New(nil),
		aijobsscraper.New(nil),
		androidjobsscraper.New(nil),
		jobicyscraper.New(nil),
		jobstreetscraper.New(nil),
		devopsjobsscraper.New(nil),
		crunchboardscraper.New(nil),
		cryptojobslistscraper.New(nil),
		workingnomadsscraper.New(nil),
		ycjobsscraper.New(nil),
		ukvisajobsscraper.New(nil),
		googlescraper.New(nil),
		adzunascraper.New(nil),
		simplyhiredscraper.New(nil),
		careerbuilderscraper.New(nil),
		dicescraper.New(nil),
		careerjetscraper.New(nil),
		monsterscraper.New(nil),
		themusescraper.New(nil),
		infojobsscraper.New(nil),
		jobsscraper.New(nil),
		snagajobscraper.New(nil),
		djinniscraper.New(nil),
		headhunterscraper.New(nil),
		mycareersfuturescraper.New(nil),
		echojobsscraper.New(nil),

		jobtechdevscraper.New(nil),
		eurojobsscraper.New(nil),
		fwdayweekscraper.New(nil),
		findworkscraper.New(nil),
		web3careerscraper.New(nil),
		arbeitsagenturscraper.New(nil),
		authenticjobsscraper.New(nil),
		// General board scrapers
		baytscraper.New(nil),
		berlinstartupjobsscraper.New(nil),
		canadajobbankscraper.New(nil),
		careeronestopscraper.New(nil),
		conservationjobsscraper.New(nil),
		coroflotscraper.New(nil),
		drupaljobsscraper.New(nil),
		exascraper.New(nil),
		fossjobsscraper.New(nil),
		francetravailscraper.New(nil),
		freelancercomscraper.New(nil),
		functionalworkscraper.New(nil),
		getonboardscraper.New(nil),
		higheredjobsscraper.New(nil),
		icrunchdatascraper.New(nil),
		jobdataapiscraper.New(nil),

		jobschscraper.New(nil),
		jobsinjapanscraper.New(nil),
		joinrisescraper.New(nil),

		nofluffjobsscraper.New(nil),
		pythonjobsscraper.New(nil),
		railsjobsscraper.New(nil),
		stepstonescraper.New(nil),
		talrooscraper.New(nil),
		teslascraper.New(nil),
		upworkscraper.New(nil),
		usajobsscraper.New(nil),

		wellfoundscraper.New(nil),
		weworkremotelyscraper.New(nil),
		wordpressjobsscraper.New(nil),
		wuzzufscraper.New(nil),
		academiccareersscraper.New(nil),
		ziprecruiterscraper.New(nil),

		// ATS scrapers (non-prefixed directories)
		adpscraper.New(nil),
		ashbyscraper.New(nil),
		avaturescraper.New(nil),
		bamboohrscraper.New(nil),
		breezyhrscraper.New(nil),
		bullhornscraper.New(nil),
		comeetscraper.New(nil),
		crelatescraper.New(nil),
		deelscraper.New(nil),
		fountainscraper.New(nil),
		freshteamscraper.New(nil),
		gemscraper.New(nil),
		hiringthingscraper.New(nil),
		icimsscraper.New(nil),
		ismartrecruitscraper.New(nil),
		jazzhrscraper.New(nil),
		jobscorescraper.New(nil),
		jobvitescraper.New(nil),
		jobylonscraper.New(nil),
		joincomscraper.New(nil),
		loxoscraper.New(nil),
		manatalscraper.New(nil),

		// ATS scrapers (ats- prefixed directories)
		atsmercorscraper.New(nil),
		atsoraclescraper.New(nil),
		atspersonioscraper.New(nil),
		atsphenomscraper.New(nil),
		atspinpointscraper.New(nil),
		atsrecruiteescraper.New(nil),
		atsrecruiterflowscraper.New(nil),
		atsripplingscraper.New(nil),
		atssmartrecruiterscraper.New(nil),
		atssuccessfactorscraper.New(nil),
		atstalentlyftscraper.New(nil),
		atstaleoscraper.New(nil),
		atsteamtailorscraper.New(nil),
		atstrakstarscraper.New(nil),
		atsukgscraper.New(nil),
		atsworkablescraper.New(nil),
		atsworkdayscraper.New(nil),

		ecojobsscraper.New(nil),
		golangjobsscraper.New(nil),
		landingjobsscraper.New(nil),
		realworkfromanywherescraper.New(nil),
	}
	m := make(map[model.Site]scraper.Scraper, len(s)+1)
	for _, sc := range s {
		m[sc.SiteName()] = sc
	}
	e := &Engine{scrapers: m, siteFailOpen: true}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

func (e *Engine) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	sites := input.Sites
	if len(sites) == 0 {
		sites = model.AllSites()
	}
	e.telemetry = RunTelemetry{Sites: make([]SiteTelemetry, 0, len(sites)), SuggestedSiteRPS: map[Site]int{}}

	// Cap initial capacity to avoid OOM when ResultsWanted <= 0 is expanded to MaxInt32
	initCap := input.ResultsWanted
	if initCap <= 0 || initCap > 100000 {
		initCap = 100000 // sane default - grows if needed
	}
	all := make([]model.JobPost, 0, initCap)
	telemetryBySite := make(map[model.Site]SiteTelemetry, len(sites))
	var allMu sync.Mutex
	var wg sync.WaitGroup
	globalSem := make(chan struct{}, globalConcurrency(input))
	siteSem := buildSiteSemaphores(input)

	type siteResult struct {
		site model.Site
		jobs []model.JobPost
		st   SiteTelemetry
		ok   bool
	}
	resultsCh := make(chan siteResult, len(sites))

	// Memory-pressure monitor (only when cap is set).
	if input.MemoryCapMB > 0 {
		memThreshold := uint64(input.MemoryCapMB) * 1024 * 1024 * 8 / 10 // 80%
		memDone := make(chan struct{})
		defer close(memDone)
		go func() {
			ticker := time.NewTicker(10 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-memDone:
					return
				case <-ticker.C:
				}
				allocMB := getHeapAllocMB()
				if uint64(allocMB)*1024*1024 > memThreshold {
					util.Warn("memory_pressure", map[string]any{
						"alloc_mb":  allocMB,
						"cap_mb":    input.MemoryCapMB,
						"pct":       uint64(allocMB) * 1024 * 1024 * 100 / (uint64(input.MemoryCapMB) * 1024 * 1024),
						"gc_cycles": readGCCycles(),
					})
					// Force GC when above 80% threshold to prevent runaway heap.
					runtime.GC()
				}
							}
		}()
	}

	for _, site := range sites {
		sc, ok := e.scrapers[site]
		if !ok {
			st := SiteTelemetry{Site: Site(site), Attempted: false, Success: false, Error: "unsupported site", FailOpenReason: "unsupported_site", StatusCodeCount: map[int]int{}}
			telemetryBySite[site] = st
			util.Warn("unsupported", map[string]any{"site": site})
			resultsCh <- siteResult{site: site, st: st, ok: false}
			continue
		}
		wg.Add(1)
		go func(site model.Site, sc scraper.Scraper) {
			defer wg.Done()
			if err := waitForMemoryBudget(ctx, input.MemoryCapMB); err != nil {
				resultsCh <- siteResult{site: site, st: SiteTelemetry{Site: Site(site), Attempted: false, Success: false, Error: err.Error(), FailOpenReason: "memory_budget_cancelled"}, ok: false}
				return
			}
			globalSem <- struct{}{}
			defer func() { <-globalSem }()
			if sem, ok := siteSem[site]; ok {
				sem <- struct{}{}
				defer func() { <-sem }()
			}

			// Check required env vars before attempting scrape.
			if vars, needsKey := requiredEnvVars[site]; needsKey {
				missing := make([]string, 0, len(vars))
				for _, ev := range vars {
					if os.Getenv(ev) == "" {
						missing = append(missing, ev)
					}
				}
				if len(missing) > 0 {
					st := SiteTelemetry{Site: Site(site), Attempted: false, Success: false, Error: fmt.Sprintf("missing env vars: %s", strings.Join(missing, ", ")), FailOpenReason: "missing_credentials"}
					allMu.Lock()
					telemetryBySite[site] = st
					allMu.Unlock()
					util.Warn("skipping", map[string]any{"site": site, "reason": "missing required env var(s)", "vars": strings.Join(missing, ", ")})
					resultsCh <- siteResult{site: site, st: st, ok: false}
					return
				}
			}

			baseInput := input
			if baseInput.SiteResultsWanted != nil {
				if n, ok := baseInput.SiteResultsWanted[site]; ok && n > 0 {
					baseInput.ResultsWanted = n
				}
			}
				if baseInput.SiteLocation != nil {
				if v := strings.TrimSpace(baseInput.SiteLocation[site]); v != "" {
					baseInput.Location = v
				}
			}
			if baseInput.SiteCountry != nil {
				if c, ok := baseInput.SiteCountry[site]; ok && c != "" {
					baseInput.Country = c
				}
			}

			// Build terms list (per-site overrides, then global SearchTerms, then single SearchTerm).
			terms := []string{strings.TrimSpace(baseInput.SearchTerm)}
			if baseInput.SiteSearch != nil {
				if vs, ok := baseInput.SiteSearch[site]; ok {
					terms = vs
				} else if len(baseInput.SearchTerms) > 0 {
					terms = baseInput.SearchTerms
				}
			} else if len(baseInput.SearchTerms) > 0 {
				terms = baseInput.SearchTerms
			}
			if len(terms) == 0 {
				terms = []string{""}
			}

			// Build locations list (per-site multi, then global Locations, then single Location).
			locs := []string{strings.TrimSpace(baseInput.Location)}
			if len(baseInput.SiteLocations) > 0 {
				if vs, ok := baseInput.SiteLocations[site]; ok && len(vs) > 0 {
					locs = vs
				}
			} else if len(baseInput.Locations) > 0 {
				locs = baseInput.Locations
			}
			if len(locs) == 0 {
				locs = []string{""}
			}
			// ponytail: skip_location — remote-only boards don't need location combos
			if baseInput.SiteSkipLocation != nil && baseInput.SiteSkipLocation[site] {
				locs = []string{""}
			}

			st := SiteTelemetry{Site: Site(site), Attempted: true, StatusCodeCount: map[int]int{}}
			aggregated := make([]model.JobPost, 0)
			var lastErr error
			siteStart := time.Now()
			method := scraper.Method(site)
			util.Info("scrape_start", map[string]any{"site": site, "method": method, "terms": len(terms), "locs": len(locs)})

			// Quick-fail if Playwright is missing for a playwright site
			if method == "playwright" && !e.playwrightCheck() {
				err := fmt.Errorf("%s: requires Playwright but Node.js or playwright module is not installed (run: npx playwright install chromium)", site)
				st.Error = err.Error()
				st.Success = false
				util.Warn("playwright_missing", map[string]any{"site": site})
				allMu.Lock()
				telemetryBySite[site] = st
				allMu.Unlock()
				resultsCh <- siteResult{site: site, st: st, ok: false}
				return
			}

			for _, term := range terms {
				for _, loc := range locs {
					siteInput := baseInput
					siteInput.SearchTerm = term
					siteInput.Location = loc
					util.Debug("scrape_try", map[string]any{"site": site, "search": siteInput.SearchTerm, "location": siteInput.Location})

					// Per-site timeout override
					scrapeCtx := ctx
					if baseInput.SiteTimeout != nil {
						if d, ok := baseInput.SiteTimeout[site]; ok && d > 0 {
							var cancel context.CancelFunc
							scrapeCtx, cancel = context.WithTimeout(ctx, d)
							defer cancel()
						}
					}

					jobs, err := sc.Scrape(scrapeCtx, siteInput)
					aggregated = append(aggregated, jobs...)

					// ponytail: streaming — push each job through JobStream when set
					if baseInput.JobStream != nil && len(jobs) > 0 {
						for i := range jobs {
							select {
							case baseInput.JobStream <- jobs[i]:
							case <-ctx.Done():
							}
						}
					}
					if err != nil {
						lastErr = err
						st.Error = err.Error()
						st.Success = false
						st.ChallengeDetected = containsAny(st.Error, "captcha", "cloudflare", "attention required", "forbidden", "blocked")
						if e.siteFailOpen {
							st.FailOpenReason = classifyFailOpenReason(err)
							util.Warn("fail_open", map[string]any{"site": site, "method": method, "reason": st.FailOpenReason, "err": st.Error, "partial": len(aggregated), "term": term, "location": loc})
						} else {
							util.Error("scrape_failed", map[string]any{"site": site, "method": method, "err": st.Error, "partial": len(aggregated), "term": term, "location": loc})
						}
						allMu.Lock()
						e.telemetry.SuggestedSiteRPS[Site(site)] = suggestRPS(input.SiteRPS[site], err)
						telemetryBySite[site] = st
						allMu.Unlock()
						// Continue to next (term, loc) combo - don't break.
						continue
					}
				}
			}
			elapsed := time.Since(siteStart)
			if lastErr != nil && len(aggregated) == 0 {
				// All (term, loc) combos failed with zero results.
				util.Warn("scrape_failed_all", map[string]any{"site": site, "method": method, "elapsed": elapsed.String(), "err": lastErr.Error()})
				resultsCh <- siteResult{site: site, st: st, ok: false}
				return
			}
			if st.Error == "" {
				st.Success = true
			}
			st.ResultCount = len(aggregated)
			if st.Success && len(aggregated) == 0 {
				st.EmptyPageRate = 1
				util.APIMiss("no_results", map[string]any{"site": site, "method": method})
			} else if len(aggregated) > 0 {
				util.Info("scrape_done", map[string]any{"site": site, "method": method, "jobs": len(aggregated), "elapsed": elapsed.String()})
			}
			allMu.Lock()
			e.telemetry.SuggestedSiteRPS[Site(site)] = suggestRPS(input.SiteRPS[site], nil)
			telemetryBySite[site] = st
			allMu.Unlock()
			normalizeIsRemote(aggregated, site, baseInput)
			if len(aggregated) > 0 || st.Success {
				resultsCh <- siteResult{site: site, jobs: aggregated, st: st, ok: true}
			}
		}(site, sc)
	}
	wg.Wait()
	close(resultsCh)

	if !e.siteFailOpen {
		for _, site := range sites {
			if st, ok := telemetryBySite[site]; ok && !st.Success && st.Error != "" {
				return nil, fmt.Errorf("scrape %s: %s", site, st.Error)
			}
		}
	}

	mxVerifier := internalemail.NewMXVerifier()
	// Create company-page enricher for jobs that have a CompanyURL set.
	// This fetches the company website and runs email extraction + MX verification
	// on the page content, finding emails not present in job descriptions.
	companyEnricher := internalemail.NewCompanyPageEnricher(
		util.NewHTTPClient(util.ClientOptions{Retries: 1, Timeout: 10 * time.Second}),
		3, // concurrency
		0, // no pause between fetches
	)
	seenGlobal := make(map[string]struct{})
	for res := range resultsCh {
		if !res.ok {
			continue
		}
		jobs := res.jobs
		for i := range jobs {
			normalizeJobPost(&jobs[i])
			// Save raw HTML before stripping - ExtractFromHTML needs it for mailto: links.
			rawHTML := jobs[i].Description
			rawCompHTML := jobs[i].CompanyDescription
			jobs[i].Description = util.StripHTML(jobs[i].Description)
			jobs[i].CompanyDescription = util.StripHTML(jobs[i].CompanyDescription)
			jobs[i].Site = string(res.site)
			now := time.Now()
			jobs[i].FetchedAt = &now
			// Run ExtractFromHTML on raw HTML BEFORE it gets stripped - this captures
			// mailto: hrefs that would be destroyed by StripHTML.
			if rawHTML != "" {
				htmlEmails := internalemail.ExtractFromHTML(rawHTML)
				for _, e := range htmlEmails {
					jobs[i].Emails = append(jobs[i].Emails, model.Email{
						Addr:   e.Addr,
						Source: "description",
						Role:   e.Role,
					})
				}
			}
			if rawCompHTML != "" {
				htmlEmails := internalemail.ExtractFromHTML(rawCompHTML)
				for _, e := range htmlEmails {
					jobs[i].Emails = append(jobs[i].Emails, model.Email{
						Addr:   e.Addr,
						Source: "company_description",
						Role:   e.Role,
					})
				}
			}
			enrichJobEmails(&jobs[i], mxVerifier, companyEnricher, ctx, input.VerifyConcurrency)

			// ponytail: EmailEnrich — auto-generate hr@company.com from domain
			// when a job has a domain but no recruiter emails were found.
			if input.EmailEnrich && len(jobs[i].Emails) == 0 && jobs[i].Domain != "" {
				if !strings.HasPrefix(jobs[i].Domain, "gmail.") &&
					!strings.HasPrefix(jobs[i].Domain, "outlook.") &&
					!strings.HasPrefix(jobs[i].Domain, "yahoo.") &&
					!strings.HasPrefix(jobs[i].Domain, "hotmail.") &&
					!strings.HasPrefix(jobs[i].Domain, "aol.") &&
					strings.Contains(jobs[i].Domain, ".") {
					for _, prefix := range []string{"hr", "careers", "recruiting", "jobs"} {
						addr := prefix + "@" + jobs[i].Domain
						if _, err := netmail.ParseAddress(addr); err == nil {
							jobs[i].Emails = append(jobs[i].Emails, model.Email{
								Addr:   addr,
								Source: "enrich",
								Role:   true, // hr/careers/recruiting/jobs are role addresses
							})
							break
						}
					}
				}
			}
			if input.EnforceAnnualSalary {
				jobs[i].Compensation = normalize.AnnualizeCompensation(jobs[i].Compensation)
			}
			jobs[i].QualityScore = quality.Score(&jobs[i])
			util.Debug("job", map[string]any{"site": res.site, "job_id": jobs[i].ID, "title": jobs[i].Title})

			// Immediate global deduplication to save memory.
			key := strings.TrimSpace(jobs[i].JobURL)
			if key == "" {
				key = strings.TrimSpace(jobs[i].ID)
			}
			if key == "" {
				continue
			}
			if _, ok := seenGlobal[key]; ok {
				continue
			}
			seenGlobal[key] = struct{}{}
			all = append(all, jobs[i])
			// Eagerly trim to ResultsWanted to prevent runaway heap growth.
			// Trim at 2x target so late-arriving higher-quality results can replace
			// early candidates; GC is forced to reclaim the backing array.
			if input.ResultsWanted > 0 && len(all) > input.ResultsWanted*2 {
				all = all[:input.ResultsWanted]
				runtime.GC()
			}
			if len(seenGlobal) > input.ResultsWanted*3 {
				seenGlobal = make(map[string]struct{}, input.ResultsWanted)
			}
		}
	}

	// Domain-level batch enrichment: visit each company's website ONCE and
	// apply found emails to all jobs from that domain. This catches company
	// contact pages (hr@, careers@) that individual job listings don't show.
	// Runs only when EmailEnrich is enabled and jobs have CompanyURLs.
	if input.EmailEnrich {
		type domainInfo struct {
			origin string
			jobs   []int
		}
		domains := make(map[string]*domainInfo)

		for idx, j := range all {
			if len(j.Emails) > 0 {
				continue
			}
			var origins []string

			// Try CompanyURL (skip if it points back to the source site)
			if j.CompanyURL != "" {
				u, err := url.Parse(j.CompanyURL)
				if err == nil {
					siteHost := strings.ToLower(strings.TrimPrefix(j.Site, "www."))
					urlHost := strings.ToLower(strings.TrimPrefix(u.Host, "www."))
					if !strings.Contains(urlHost, siteHost) && !strings.Contains(siteHost, urlHost) {
						origins = append(origins, u.Scheme+"://"+u.Host)
					}
				}
			}

			// Try deriving domain from company name (multiple TLDs)
			if name := strings.TrimSpace(j.CompanyName); name != "" {
				slug := strings.ToLower(name)
				slug = strings.NewReplacer(" ", "", ".", "", "-", "", "&", "", ",", "").Replace(slug)
				for _, tld := range []string{".com", ".io", ".co", ".org"} {
					if _, err := net.DefaultResolver.LookupHost(ctx, slug+tld); err == nil {
						candidate := "https://" + slug + tld
						origins = append(origins, candidate)
						all[idx].CompanyURL = candidate
						break // first resolving TLD wins
					}
				}
			}

			for _, origin := range origins {
				if info, ok := domains[origin]; ok {
					info.jobs = append(info.jobs, idx)
				} else {
					domains[origin] = &domainInfo{origin: origin, jobs: []int{idx}}
				}
			}
		}

		if len(domains) > 0 {
			mpe := internalemail.NewMultiPageCompanyEnricher(
				util.NewHTTPClient(util.ClientOptions{Retries: 1, Timeout: 10 * time.Second}),
				3, 50,
			)
			// Cap domains to prevent OOM on large scrapes with thousands of unique companies
		const maxEnrichDomains = 200
		enriched := 0
		for origin, info := range domains {
			if enriched >= maxEnrichDomains {
				break
			}
				if ctx.Err() != nil {
					break
				}
				emails, err := mpe.Enrich(ctx, origin)
				if err != nil || len(emails) == 0 {
					continue
				}
				util.Info("domain_enrich", map[string]any{
					"origin": origin, "emails": len(emails),
					"jobs": len(info.jobs),
				})
				enriched++
				for _, idx := range info.jobs {
					for _, e := range emails {
						all[idx].Emails = append(all[idx].Emails, model.Email{
							Addr:   e.Addr,
							Source: "domain_enrich",
							Role:   e.Role,
						})
					}
					all[idx].Emails = dedupEmails(all[idx].Emails)
				}
			}
		}

		// Second EmailEnrich pass: jobs that gained a CompanyURL from domain heuristic
		// but the website had no contact page. Generate hr@{domain} as fallback.
		for idx, j := range all {
			if len(j.Emails) > 0 || j.CompanyURL == "" {
				continue
			}
			domain := j.Domain
			if domain == "" {
				if u, err := url.Parse(j.CompanyURL); err == nil && u.Host != "" {
					domain = strings.TrimPrefix(u.Host, "www.")
				}
			}
			if domain == "" || !strings.Contains(domain, ".") {
				continue
			}
			if strings.HasPrefix(domain, "gmail.") ||
				strings.HasPrefix(domain, "outlook.") ||
				strings.HasPrefix(domain, "yahoo.") ||
				strings.HasPrefix(domain, "hotmail.") ||
				strings.HasPrefix(domain, "aol.") {
				continue
			}
			for _, prefix := range []string{"hr", "careers", "recruiting", "jobs"} {
				addr := prefix + "@" + domain
				if _, err := netmail.ParseAddress(addr); err == nil {
					all[idx].Emails = append(all[idx].Emails, model.Email{
						Addr:   addr,
						Source: "enrich",
						Role:   true,
					})
					break
				}
			}
		}
	}

	for _, site := range sites {
		if st, ok := telemetryBySite[site]; ok {
			e.telemetry.Sites = append(e.telemetry.Sites, st)
		}
	}

	// Apply filters: score and emails.
	if input.MinScore > 0 {
		filtered := all[:0]
		for _, j := range all {
			if j.QualityScore >= input.MinScore {
				filtered = append(filtered, j)
			}
		}
		all = filtered
	}
	if input.EmailsOnly {
		filtered := all[:0]
		for _, j := range all {
			if len(j.Emails) > 0 {
				filtered = append(filtered, j)
			}
		}
		all = filtered
	}
	// Apply recency filters: HoursOld (relative) and SinceDate (absolute).
	if input.HoursOld > 0 || input.SinceDate != "" {
		var cutoff time.Time
		if input.HoursOld > 0 {
			cutoff = time.Now().Add(-time.Duration(input.HoursOld) * time.Hour)
		}
		if input.SinceDate != "" {
			since, err := parseSinceDate(input.SinceDate)
			if err == nil && (cutoff.IsZero() || since.After(cutoff)) {
				cutoff = since
			}
		}
		filtered := all[:0]
		for _, j := range all {
			if j.DatePosted != nil && !j.DatePosted.IsZero() && j.DatePosted.After(cutoff) {
				filtered = append(filtered, j)
			}
		}
		all = filtered
	}

	sort.SliceStable(all, func(i, j int) bool { return all[i].ID < all[j].ID })

	// ponytail: title+company dedup — catch same job posted on different sites
	if input.DedupByCompany || input.Dedup {
		seenNormalized := make(map[string]bool, len(all))
		filtered := all[:0]
		for _, j := range all {
			key := strings.ToLower(strings.TrimSpace(j.Title + "|" + j.CompanyName))
			if key == "|" {
				filtered = append(filtered, j)
				continue
			}
			if seenNormalized[key] {
				continue
			}
			seenNormalized[key] = true
			filtered = append(filtered, j)
		}
		all = filtered
	}
	if input.ResultsWanted > 0 && len(all) > input.ResultsWanted {
		all = all[:input.ResultsWanted]
	}
	return all, nil
}

// readGCCycles queries the Go runtime for completed GC cycles.
// Handles both uint64 and float64 return kinds across Go versions.
func readGCCycles() uint64 {
	sample := make([]metrics.Sample, 1)
	sample[0].Name = "/gc/cycles/automatic:gc-cycle"
	metrics.Read(sample)
	switch sample[0].Value.Kind() {
	case metrics.KindUint64:
		return sample[0].Value.Uint64()
	case metrics.KindFloat64:
		return uint64(sample[0].Value.Float64())
	default:
		return 0
	}
}

// waitForMemoryBudget blocks new scrape launches while heap usage is above
// ~90% of configured memory cap. It resumes once usage drops below ~75%.
func waitForMemoryBudget(ctx context.Context, capMB int) error {
	if capMB <= 0 {
		return nil
	}
	capBytes := uint64(capMB) * 1024 * 1024
	hard := capBytes * 90 / 100
	resume := capBytes * 75 / 100
	warned := false

	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		allocBytes := uint64(getHeapAllocMB()) * 1024 * 1024
		if allocBytes <= hard {
			return nil
		}
		if !warned {
			util.Warn("memory_throttle", map[string]any{
				"alloc_mb": getHeapAllocMB(),
				"cap_mb":   capMB,
				"hard_pct": 90,
			})
			warned = true
		}
		runtime.GC()
		// Re-read heap usage after GC — the pre-GC value is no longer relevant.
		allocMB := getHeapAllocMB()
		if uint64(allocMB)*1024*1024 <= resume {
			return nil
		}
		if err := util.SleepWithContext(ctx, 750*time.Millisecond); err != nil {
			return err
		}
	}
}

func getHeapAllocMB() int {
	sample := make([]metrics.Sample, 1)
	sample[0].Name = "/memory/classes/heap/objects:bytes"
	metrics.Read(sample)
	if sample[0].Value.Uint64() > 0 {
		return int(sample[0].Value.Uint64() / (1024 * 1024))
	}
	return 0
}

func suggestRPS(current int, err error) int {
	if current <= 0 {
		current = 3
	}
	if err == nil {
		if current < 10 {
			return current + 1
		}
		return current
	}
	e := err.Error()
	if containsAny(e, "429", "rate", "too many requests", "captcha") {
		if current > 1 {
			return current - 1
		}
		return 1
	}
	return current
}

func containsAny(s string, vals ...string) bool {
	s = strings.ToLower(s)
	for _, v := range vals {
		if strings.Contains(s, strings.ToLower(v)) {
			return true
		}
	}
	return false
}

func globalConcurrency(input model.ScraperInput) int {
	// Scale concurrency based on memory cap if set.
	if input.MemoryCapMB > 0 {
		// Scale down under heap pressure
		if allocMB := getHeapAllocMB(); allocMB > 0 && uint64(allocMB)*100/uint64(input.MemoryCapMB) > 60 {
			return 2
		}
		switch {
		case input.MemoryCapMB <= 256:
			return 3
		case input.MemoryCapMB <= 512:
			return 5
		case input.MemoryCapMB <= 1024:
			return 8
		default:
			return 12
		}
	}
	if input.MaxRPS > 0 {
		if input.MaxRPS < 2 {
			return 2
		}
		if input.MaxRPS > 16 {
			return 16
		}
		return input.MaxRPS
	}
	return 8
}

func buildSiteSemaphores(input model.ScraperInput) map[model.Site]chan struct{} {
	out := make(map[model.Site]chan struct{})
	for site, rps := range input.SiteRPS {
		capN := rps
		if capN <= 0 {
			capN = 1
		}
		if capN > 8 {
			capN = 8
		}
		out[site] = make(chan struct{}, capN)
	}
	return out
}

func classifyFailOpenReason(err error) string {
	if err == nil {
		return ""
	}
	e := strings.ToLower(err.Error())
	switch {
	case containsAny(e, "captcha", "cloudflare", "attention required", "bot", "blocked", "datadome"):
		return "challenge_detected"
	case containsAny(e, "429", "too many requests", "rate"):
		return "rate_limited"
	case containsAny(e, "403", "401", "forbidden", "unauthorized"):
		return "access_denied"
	case containsAny(e, "timeout", "deadline exceeded"):
		return "timeout"
	default:
		return "unknown"
	}
}

func enrichJobEmails(job *model.JobPost, verifier *internalemail.MXVerifier, enricher *internalemail.CompanyPageEnricher, ctx context.Context, verifyConcurrency int) {
	// Start by deduplicating any emails already set by the scraper or from HTML extraction.
	job.Emails = dedupEmails(job.Emails)

	// Extract emails from description + company description text.
	text := jobTextForEmailExtraction(job)
	if text != "" {
		found := internalemail.ExtractFromHTML(text)
		for _, e := range found {
			job.Emails = append(job.Emails, model.Email{
				Addr:   e.Addr,
				Source: e.Source,
				Role:   e.Role,
			})
		}
		job.Emails = dedupEmails(job.Emails)
	}

	// Populate Domain from first email if not already set.
	if job.Domain == "" && len(job.Emails) > 0 {
		if d := internalemail.DomainFrom(job.Emails[0].Addr); d != "" {
			job.Domain = d
		}
	}

	// If we still have no domain, try to extract it from CompanyURL.
	if job.Domain == "" && job.CompanyURL != "" {
		if u, err := url.Parse(job.CompanyURL); err == nil && u.Host != "" {
			job.Domain = u.Host
		}
	}

	// Company-page enrichment: if the job has a CompanyURL and at least one domain,
	// fetch the company page and extract emails from it. This catches emails that
	// are on the company's careers/contact page but not in the job description.
	// Only runs when MX verification is enabled (verifyConcurrency > 0).
	if enricher != nil && job.CompanyURL != "" && verifyConcurrency > 0 && ctx.Err() == nil {
		companyEmails, err := enricher.Enrich(ctx, job.CompanyURL)
		if err == nil && len(companyEmails) > 0 {
			for _, e := range companyEmails {
				job.Emails = append(job.Emails, model.Email{
					Addr:   e.Addr,
					Source: "company_page",
					Role:   e.Role,
				})
			}
			job.Emails = dedupEmails(job.Emails)
			// Re-derive Domain if we now have richer email data.
			if len(job.Emails) > 0 {
				if d := internalemail.DomainFrom(job.Emails[0].Addr); d != "" {
					job.Domain = d
				}
			}
		}
	}

	// Run MX verification on every email with bounded concurrency.
	// 0 = skip MX verification (faster, no DNS timeouts).
	if verifier != nil && verifyConcurrency > 0 {
		sem := make(chan struct{}, verifyConcurrency)
		var mu sync.Mutex
		var wg sync.WaitGroup
		for i := range job.Emails {
			if ctx.Err() != nil {
				return
			}
			wg.Add(1)
			sem <- struct{}{}
			go func(idx int) {
				defer wg.Done()
				defer func() { <-sem }()
				verified, _ := verifier.VerifyEmail(ctx, job.Emails[idx].Addr)
				mu.Lock()
				job.Emails[idx].Verified = verified
				mu.Unlock()
			}(i)
		}
		wg.Wait()
	} else {
		for i := range job.Emails {
			job.Emails[i].Verified = false
		}
	}
}

func jobTextForEmailExtraction(job *model.JobPost) string {
	parts := make([]string, 0, 2)
	if v := strings.TrimSpace(job.Description); v != "" {
		parts = append(parts, v)
	}
	if v := strings.TrimSpace(job.CompanyDescription); v != "" {
		parts = append(parts, v)
	}
	return strings.Join(parts, "\n")
}

func dedupEmails(in []model.Email) []model.Email {
	seen := make(map[string]struct{}, len(in))
	out := make([]model.Email, 0, len(in))
	for _, e := range in {
		addr := strings.TrimSpace(strings.ToLower(e.Addr))
		if addr == "" {
			continue
		}
		if _, ok := seen[addr]; ok {
			continue
		}
		seen[addr] = struct{}{}
		e.Addr = addr
		out = append(out, e)
	}
	return out
}

// normalizeJobPost ensures fields that consumers expect are never nil/empty
// when they should have a predictable zero value.
func normalizeJobPost(j *model.JobPost) {
	if j.Skills == nil {
		j.Skills = []string{}
	}
}

func dedupWithinSite(in []model.JobPost) []model.JobPost {
	seen := map[string]struct{}{}
	out := make([]model.JobPost, 0, len(in))
	for _, j := range in {
		key := strings.TrimSpace(j.JobURL)
		if key == "" {
			key = strings.TrimSpace(j.ID)
		}
		if key == "" {
			out = append(out, j)
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, j)
	}
	return out
}

// parseSinceDate parses a --since flag value, accepting RFC3339 or YYYY-MM-DD.
func parseSinceDate(s string) (time.Time, error) {
	// Try RFC3339 first (full datetime with timezone)
	t, err := time.Parse(time.RFC3339, s)
	if err == nil {
		return t, nil
	}
	// Try YYYY-MM-DD (start of day)
	t, err = time.Parse("2006-01-02", s)
	if err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("invalid --since date %q: expected RFC3339 or YYYY-MM-DD", s)
}

// ── Public API Wrapper ──────────────────────────────────────────

// ScrapeJobs is the public-facing scrape method.
// ScraperInput and JobPost are type aliases to model types, so no conversion needed.
func (e *Engine) ScrapeJobs(ctx context.Context, input ScraperInput) ([]JobPost, error) {
	return e.Scrape(ctx, input)
}

// ScrapeJobsFull returns jobs plus per-site result metadata in one call.
// Use this when you need to know which sites succeeded/failed and why.
func (e *Engine) ScrapeJobsFull(ctx context.Context, input ScraperInput) (*ScrapeResult, error) {
	jobs, err := e.Scrape(ctx, input)
	if err != nil {
		return nil, err
	}
	sites := make([]SiteResult, len(e.telemetry.Sites))
	for i, st := range e.telemetry.Sites {
		sr := SiteResult{
			Site: st.Site,
			Jobs: st.ResultCount,
		}
		if st.Error != "" {
			err := fmt.Errorf("%s", st.Error)
			sr.Error = st.Error
			sr.Kind = ErrorKind(err)
		}
		sites[i] = sr
	}
	return &ScrapeResult{Jobs: jobs, Sites: sites}, nil
}

// ScrapeJobsStream scrapes all sites and calls cb for each job as it arrives.
// Blocks until all sites complete. Useful for long-running scrapes where you
// want progressive results rather than waiting minutes for everything.
func (e *Engine) ScrapeJobsStream(ctx context.Context, input ScraperInput, cb func(JobPost)) error {
	ch := make(chan JobPost, 1000)
	input.JobStream = ch

	// Run scrape in background
	type scrapeOut struct {
		err  error
	}
	resultCh := make(chan scrapeOut, 1)
	go func() {
		_, err := e.Scrape(ctx, input)
		resultCh <- scrapeOut{err: err}
		close(ch)
	}()

	// Drain the stream channel, calling cb for each job
	for j := range ch {
		cb(j)
	}

	res := <-resultCh
	return res.err
}

// playwrightCheck returns true if Node.js can resolve the playwright module.
// Uses sync.Once to cache the result after the first check.
func (e *Engine) playwrightCheck() bool {
	e.playwrightCachedOnce.Do(func() {
		ok := true
		out, err := exec.Command("node", "-e", "require('playwright')").CombinedOutput()
		if err != nil {
			ok = false
		}
		_ = out
		e.playwrightCached = &ok
	})
	return e.playwrightCached != nil && *e.playwrightCached
}

// ReloadConfig re-reads the config file that was passed to WithConfig.
// Useful for long-running consumers (e.g. JobHunter) that want to pick up
// config changes without restarting.
func (e *Engine) ReloadConfig() error {
	if e.configPath == "" {
		return nil // no config was set
	}
	data, err := os.ReadFile(e.configPath)
	if err != nil {
		return fmt.Errorf("reload config: %w", err)
	}
	var cfg struct {
		Defaults struct {
			Search   []string `toml:"search"`
			Location []string `toml:"location"`
		} `toml:"defaults"`
		Sites map[string]struct {
			Search   []string `toml:"search"`
			Location []string `toml:"location"`
			Country  string   `toml:"country"`
		} `toml:"sites"`
	}
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("reload config parse: %w", err)
	}
	e.configSearch = cfg.Defaults.Search
	e.configLocation = cfg.Defaults.Location
	e.configSites = cfg.Sites
	return nil
}

// normalizeIsRemote sets IsRemote on jobs based on site, location, and input signals.
func normalizeIsRemote(jobs []model.JobPost, site model.Site, input model.ScraperInput) {
	if input.RemoteOnly {
		for i := range jobs {
			jobs[i].IsRemote = true
		}
		return
	}

	// Site-level: remote-only boards
	siteStr := strings.ToLower(string(site))
	isRemoteSite := strings.Contains(siteStr, "remote") ||
		siteStr == "weworkremotely" ||
		siteStr == "workingnomads" ||
		siteStr == "4dayweek"

	for i := range jobs {
		if isRemoteSite {
			jobs[i].IsRemote = true
			continue
		}
		if jobs[i].IsRemote {
			continue // already set by scraper
		}
		// Location-level: check for "remote" in location fields
		loc := jobs[i].Location
		jobs[i].IsRemote = strings.Contains(strings.ToLower(loc.City+" "+loc.State+" "+loc.Country), "remote")
	}
}
