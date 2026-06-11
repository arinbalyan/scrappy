package scrappy

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"runtime"
	"runtime/metrics"
	"sort"
	"strings"
	"sync"
	"time"

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
}

type PostProcessor func(context.Context, *model.JobPost) error

type Engine struct {
	scrapers     map[model.Site]scraper.Scraper
	hooks        []PostProcessor
	telemetry    RunTelemetry
	siteFailOpen bool
}

func NewEngine() *Engine {
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
	return &Engine{scrapers: m, siteFailOpen: true}
}

func (e *Engine) SetSiteFailOpen(enabled bool) {
	e.siteFailOpen = enabled
}

func (e *Engine) Telemetry() RunTelemetry {
	return e.telemetry
}

func (e *Engine) RegisterHook(h PostProcessor) {
	e.hooks = append(e.hooks, h)
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

			st := SiteTelemetry{Site: Site(site), Attempted: true, StatusCodeCount: map[int]int{}}
			aggregated := make([]model.JobPost, 0)
			var lastErr error
			for _, term := range terms {
				for _, loc := range locs {
					siteInput := baseInput
					siteInput.SearchTerm = term
					siteInput.Location = loc
					util.Info("Scraping", map[string]any{"site": site, "search": siteInput.SearchTerm, "location": siteInput.Location})
					jobs, err := sc.Scrape(ctx, siteInput)
					aggregated = append(aggregated, jobs...)
					if err != nil {
						lastErr = err
						st.Error = err.Error()
						st.Success = false
						st.ChallengeDetected = containsAny(st.Error, "captcha", "cloudflare", "attention required", "forbidden", "blocked")
						if e.siteFailOpen {
							st.FailOpenReason = classifyFailOpenReason(err)
							util.Warn("fail_open", map[string]any{"site": site, "reason": st.FailOpenReason, "err": st.Error, "partial": len(aggregated), "term": term, "location": loc})
						} else {
							util.Error("scrape_failed", map[string]any{"site": site, "err": st.Error, "partial": len(aggregated), "term": term, "location": loc})
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
			if lastErr != nil && len(aggregated) == 0 {
				// All (term, loc) combos failed with zero results.
				resultsCh <- siteResult{site: site, st: st, ok: false}
				return
			}
			if st.Error == "" {
				st.Success = true
			}
			st.ResultCount = len(aggregated)
			if st.Success && len(aggregated) == 0 {
				st.EmptyPageRate = 1
				util.APIMiss("no_results", map[string]any{"site": site})
			}
			if st.Error == "" || len(aggregated) > 0 {
				util.Info("scraped", map[string]any{"site": site, "jobs": len(aggregated)})
			}
			allMu.Lock()
			e.telemetry.SuggestedSiteRPS[Site(site)] = suggestRPS(input.SiteRPS[site], nil)
			telemetryBySite[site] = st
			allMu.Unlock()
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
			if input.EnforceAnnualSalary {
				jobs[i].Compensation = normalize.AnnualizeCompensation(jobs[i].Compensation)
			}
			jobs[i].QualityScore = quality.Score(&jobs[i])
			for _, h := range e.hooks {
				if err := h(ctx, &jobs[i]); err != nil {
					if e.siteFailOpen {
						util.Warn("hook_failed", map[string]any{"site": res.site, "job_id": jobs[i].ID, "err": err.Error()})
						continue
					}
					return nil, fmt.Errorf("post-process %s: %w", jobs[i].ID, err)
				}
			}
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
		found := internalemail.Extract(text)
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
	if enricher != nil && job.CompanyURL != "" && ctx.Err() == nil {
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
	if verifier != nil {
		if verifyConcurrency <= 0 {
			verifyConcurrency = 5 // default
		}
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
// It accepts and returns public types so external consumers like JobHunter
// can use it without importing internal packages.
func (e *Engine) ScrapeJobs(ctx context.Context, input ScraperInput) ([]JobPost, error) {
	internalJobs, err := e.Scrape(ctx, scraperInputToModel(input))
	if err != nil {
		return nil, err
	}
	return jobPostsFromModel(internalJobs), nil
}

// ── Type Conversions ────────────────────────────────────────────

func scraperInputToModel(in ScraperInput) model.ScraperInput {
	sites := make([]model.Site, len(in.Sites))
	for i, s := range in.Sites {
		sites[i] = model.Site(s)
	}
	out := model.ScraperInput{
		Sites:               sites,
		SearchTerm:          in.SearchTerm,
		Location:            in.Location,
		Country:             model.Country(in.Country),
		IsRemote:            in.IsRemote,
		JobType:             model.JobType(in.JobType),
		EasyApply:           in.EasyApply,
		ResultsWanted:       in.ResultsWanted,
		HoursOld:            in.HoursOld,
		SinceDate:           in.SinceDate,
		DescriptionFormat:   in.DescriptionFormat,
		EnforceAnnualSalary: in.EnforceAnnualSalary,
		EmailsOnly:          in.EmailsOnly,
		MinScore:            in.MinScore,
		RemoteOnly:          in.RemoteOnly,
		VerifyEmail:         in.VerifyEmail,
		VerifyConcurrency:   in.VerifyConcurrency,
		Proxy:               in.Proxy,
		MemoryCapMB:         in.MemoryCapMB,
		SearchTerms:         in.SearchTerms,
		Locations:           in.Locations,
		MaxRPS:              in.MaxRPS,
		LogLevel:            in.LogLevel,
	}
	if in.SiteSearch != nil {
		out.SiteSearch = make(map[model.Site][]string, len(in.SiteSearch))
		for k, v := range in.SiteSearch {
			out.SiteSearch[model.Site(k)] = v
		}
	}
	if in.SiteLocations != nil {
		out.SiteLocations = make(map[model.Site][]string, len(in.SiteLocations))
		for k, v := range in.SiteLocations {
			out.SiteLocations[model.Site(k)] = v
		}
	}
	if in.SiteResultsWanted != nil {
		out.SiteResultsWanted = make(map[model.Site]int, len(in.SiteResultsWanted))
		for k, v := range in.SiteResultsWanted {
			out.SiteResultsWanted[model.Site(k)] = v
		}
	}
	if in.SiteRPS != nil {
		out.SiteRPS = make(map[model.Site]int, len(in.SiteRPS))
		for k, v := range in.SiteRPS {
			out.SiteRPS[model.Site(k)] = v
		}
	}
	return out
}

func jobPostsFromModel(in []model.JobPost) []JobPost {
	out := make([]JobPost, len(in))
	for i, j := range in {
		out[i] = jobPostFromModel(j)
	}
	return out
}

func jobPostFromModel(j model.JobPost) JobPost {
	return JobPost{
		ID:                 j.ID,
		Title:              j.Title,
		CompanyName:        j.CompanyName,
		CompanyURL:         j.CompanyURL,
		JobURL:             j.JobURL,
		JobURLDirect:       j.JobURLDirect,
		Location:           locationFromModel(j.Location),
		IsRemote:           j.IsRemote,
		Description:        j.Description,
		JobType:            j.JobType,
		DatePosted:         j.DatePosted,
		Site:               Site(j.Site),
		FetchedAt:          j.FetchedAt,
		Emails:             emailsFromModel(j.Emails),
		Compensation:       compFromModel(j.Compensation),
		Seniority:          j.Seniority,
		Department:         j.Department,
		Domain:             j.Domain,
		Industry:           j.Industry,
		CompanyLogoURL:     j.CompanyLogoURL,
		ApplyMethod:        j.ApplyMethod,
		JobLevel:           j.JobLevel,
		CompanyIndustry:    j.CompanyIndustry,
		CompanyDescription: j.CompanyDescription,
		Skills:             j.Skills,
		ExperienceRange:    j.ExperienceRange,
		QualityScore:       j.QualityScore,
	}
}

func locationFromModel(l model.Location) Location {
	return Location{
		City:    l.City,
		State:   l.State,
		Country: l.Country,
	}
}

func emailsFromModel(in []model.Email) []Email {
	out := make([]Email, len(in))
	for i, e := range in {
		out[i] = Email{
			Addr:     e.Addr,
			Verified: e.Verified,
			Source:   e.Source,
			Role:     e.Role,
		}
	}
	return out
}

func compFromModel(c *model.Compensation) *Compensation {
	if c == nil {
		return nil
	}
	return &Compensation{
		Interval:  CompensationInterval(c.Interval),
		MinAmount: c.MinAmount,
		MaxAmount: c.MaxAmount,
		Currency:  c.Currency,
	}
}
