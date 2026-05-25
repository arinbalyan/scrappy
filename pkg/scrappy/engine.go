package scrappy

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	htmlparser "golang.org/x/net/html"
	"github.com/arinbalyan/scrappy/internal/dedup"
	internalemail "github.com/arinbalyan/scrappy/internal/email"
	"github.com/arinbalyan/scrappy/internal/model"
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
	glassdoorscraper "github.com/arinbalyan/scrappy/internal/scraper/glassdoor"
	googlescraper "github.com/arinbalyan/scrappy/internal/scraper/google"
	dribbbleScraper "github.com/arinbalyan/scrappy/internal/scraper/dribbble"
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
	jobstreetscraper "github.com/arinbalyan/scrappy/internal/scraper/jobstreet"
	jobindexscraper "github.com/arinbalyan/scrappy/internal/scraper/jobindex"
	jobspressoscraper "github.com/arinbalyan/scrappy/internal/scraper/jobspresso"
	larajobsscraper "github.com/arinbalyan/scrappy/internal/scraper/larajobs"
	linkedinscraper "github.com/arinbalyan/scrappy/internal/scraper/linkedin"
	naukriscraper "github.com/arinbalyan/scrappy/internal/scraper/naukri"
	remotefirstjobsscraper "github.com/arinbalyan/scrappy/internal/scraper/remotefirstjobs"
	remoteokscraper "github.com/arinbalyan/scrappy/internal/scraper/remoteok"
	reedscraper "github.com/arinbalyan/scrappy/internal/scraper/reed"
	remotivescraper "github.com/arinbalyan/scrappy/internal/scraper/remotive"
	startupjobsscraper "github.com/arinbalyan/scrappy/internal/scraper/startupjobs"

	ukvisajobsscraper "github.com/arinbalyan/scrappy/internal/scraper/ukvisajobs"
	vuejobsscraper "github.com/arinbalyan/scrappy/internal/scraper/vuejobs"
	workingnomadsscraper "github.com/arinbalyan/scrappy/internal/scraper/workingnomads"
	ycjobsscraper "github.com/arinbalyan/scrappy/internal/scraper/ycjobs"
	adzunascraper "github.com/arinbalyan/scrappy/internal/scraper/adzuna"
	simplyhiredscraper "github.com/arinbalyan/scrappy/internal/scraper/simplyhired"
	careerbuilderscraper "github.com/arinbalyan/scrappy/internal/scraper/careerbuilder"
	dicescraper "github.com/arinbalyan/scrappy/internal/scraper/dice"
	echojobsscraper "github.com/arinbalyan/scrappy/internal/scraper/echojobs"
	nodeskscraper "github.com/arinbalyan/scrappy/internal/scraper/nodesk"
	jobtechdevscraper "github.com/arinbalyan/scrappy/internal/scraper/jobtechdev"
	careerjetscraper "github.com/arinbalyan/scrappy/internal/scraper/careerjet"
	jooblescraper "github.com/arinbalyan/scrappy/internal/scraper/jooble"
	monsterscraper "github.com/arinbalyan/scrappy/internal/scraper/monster"
	themusescraper "github.com/arinbalyan/scrappy/internal/scraper/themuse"
	infojobsscraper "github.com/arinbalyan/scrappy/internal/scraper/infojobs"
	jobsscraper "github.com/arinbalyan/scrappy/internal/scraper/jobsdb"
	snagajobscraper "github.com/arinbalyan/scrappy/internal/scraper/snagajob"
	djinniscraper "github.com/arinbalyan/scrappy/internal/scraper/djinni"
	headhunterscraper "github.com/arinbalyan/scrappy/internal/scraper/headhunter"
	mycareersfuturescraper "github.com/arinbalyan/scrappy/internal/scraper/mycareersfuture"
	eurojobsscraper "github.com/arinbalyan/scrappy/internal/scraper/eurojobs"
	fwdayweekscraper "github.com/arinbalyan/scrappy/internal/scraper/4dayweek"
	findworkscraper "github.com/arinbalyan/scrappy/internal/scraper/findwork"
	web3careerscraper "github.com/arinbalyan/scrappy/internal/scraper/web3career"

	arbeitsagenturscraper "github.com/arinbalyan/scrappy/internal/scraper/arbeitsagentur"
	authenticjobsscraper "github.com/arinbalyan/scrappy/internal/scraper/authenticjobs"
	ecojobsscraper "github.com/arinbalyan/scrappy/internal/scraper/ecojobs"
	golangjobsscraper "github.com/arinbalyan/scrappy/internal/scraper/golangjobs"
	landingjobsscraper "github.com/arinbalyan/scrappy/internal/scraper/landingjobs"
	realworkfromanywherescraper "github.com/arinbalyan/scrappy/internal/scraper/realworkfromanywhere"
	"github.com/arinbalyan/scrappy/internal/util"
)

// requiredEnvVars maps sites to environment variables that must be set
// before the scraper can function.  The engine skips these sites with
// a clear WARN message instead of wasting time on a doomed request.
var requiredEnvVars = map[model.Site][]string{
	model.SiteAdzuna:        {"ADZUNA_APP_ID", "ADZUNA_APP_KEY"},
	model.SiteCareerjet:     {"CAREERJET_AFFID"},
	model.SiteInfoJobs:      {"INFOJOBS_CLIENT_ID", "INFOJOBS_CLIENT_SECRET"},
	model.SiteFindwork:      {"FINDWORK_API_KEY"},
	model.SiteArbeitsagentur: {"ARBEITSAGENTUR_API_KEY"},
	model.SiteWeb3Career:     {"WEB3CAREER_API_TOKEN"},
	model.SiteJobTechDev:    {"JOBTECHDEV_API_KEY"},
	model.SiteAuthenticJobs: {"AUTHENTICJOBS_API_KEY"},
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
		naukriscraper.New(nil),
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
		vuejobsscraper.New(nil),
		larajobsscraper.New(nil),
		arbeitnowscraper.New(nil),
		hackernewsscraper.New(nil),
		cryptocurrencyjobsscraper.New(nil),
		dribbbleScraper.New(nil),
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
		glassdoorscraper.New(nil),
		googlescraper.New(nil),
		adzunascraper.New(nil),
		simplyhiredscraper.New(nil),
		careerbuilderscraper.New(nil),
		dicescraper.New(nil),
		careerjetscraper.New(nil),
		jooblescraper.New(nil),
		monsterscraper.New(nil),
		themusescraper.New(nil),
		infojobsscraper.New(nil),
		jobsscraper.New(nil),
		snagajobscraper.New(nil),
		djinniscraper.New(nil),
		headhunterscraper.New(nil),
		mycareersfuturescraper.New(nil),
		echojobsscraper.New(nil),
		nodeskscraper.New(nil),
		jobtechdevscraper.New(nil),
		eurojobsscraper.New(nil),
		fwdayweekscraper.New(nil),
		findworkscraper.New(nil),
		web3careerscraper.New(nil),
		arbeitsagenturscraper.New(nil),
		authenticjobsscraper.New(nil),
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
	e.telemetry = RunTelemetry{Sites: make([]SiteTelemetry, 0, len(sites)), SuggestedSiteRPS: map[model.Site]int{}}

	all := make([]model.JobPost, 0)
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
				var m runtime.MemStats
				runtime.ReadMemStats(&m)
				if m.Alloc > memThreshold {
					util.Warn("memory_pressure", map[string]any{
						"alloc_mb":   m.Alloc / 1024 / 1024,
						"cap_mb":     input.MemoryCapMB,
						"pct":        m.Alloc * 100 / (uint64(input.MemoryCapMB) * 1024 * 1024),
						"gc_cycles":  m.NumGC,
					})
				}
			}
		}()
	}

	for _, site := range sites {
		sc, ok := e.scrapers[site]
		if !ok {
			st := SiteTelemetry{Site: site, Attempted: false, Success: false, Error: "unsupported site", FailOpenReason: "unsupported_site", StatusCodeCount: map[int]int{}}
			telemetryBySite[site] = st
			util.Warn("unsupported", map[string]any{"site": site})
			resultsCh <- siteResult{site: site, st: st, ok: false}
			continue
		}
		wg.Add(1)
		go func(site model.Site, sc scraper.Scraper) {
			defer wg.Done()
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
					st := SiteTelemetry{Site: site, Attempted: false, Success: false, Error: fmt.Sprintf("missing env vars: %s", strings.Join(missing, ", ")), FailOpenReason: "missing_credentials"}
					allMu.Lock()
					telemetryBySite[site] = st
					allMu.Unlock()
					util.Warn("skipping", map[string]any{"site": site, "reason": "missing required env var(s)", "vars": strings.Join(missing, ", ")})
					resultsCh <- siteResult{site: site, st: st, ok: false}
					return
				}
			}

			baseInput := input
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

			st := SiteTelemetry{Site: site, Attempted: true, StatusCodeCount: map[int]int{}}
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
						e.telemetry.SuggestedSiteRPS[site] = suggestRPS(input.SiteRPS[site], err)
						telemetryBySite[site] = st
						allMu.Unlock()
						// Continue to next (term, loc) combo — don't break.
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
			e.telemetry.SuggestedSiteRPS[site] = suggestRPS(input.SiteRPS[site], nil)
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

	processedBySite := make(map[model.Site][]model.JobPost, len(sites))
	mxVerifier := internalemail.NewMXVerifier()
	for res := range resultsCh {
		if !res.ok {
			continue
		}
		jobs := res.jobs
		for i := range jobs {
			jobs[i].Description = stripHTML(jobs[i].Description)
			jobs[i].CompanyDescription = stripHTML(jobs[i].CompanyDescription)
			jobs[i].Site = string(res.site)
			now := time.Now()
			jobs[i].FetchedAt = &now
			enrichJobEmails(&jobs[i], mxVerifier, ctx)
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
		}
		processedBySite[res.site] = dedupWithinSite(jobs)
	}

	for _, site := range sites {
		if st, ok := telemetryBySite[site]; ok {
			e.telemetry.Sites = append(e.telemetry.Sites, st)
		}
		if jobs, ok := processedBySite[site]; ok {
			all = append(all, jobs...)
		}
	}

	all = dedup.DedupFilters(all, !input.Dedup, input.DedupByCompany, true)
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
	if input.HoursOld > 0 {
		cutoff := time.Now().Add(-time.Duration(input.HoursOld) * time.Hour)
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

func enrichJobEmails(job *model.JobPost, verifier *internalemail.MXVerifier, ctx context.Context) {
	job.Emails = dedupEmails(job.Emails)

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

	// Run MX verification to set Verified field on each email.
	if verifier != nil {
		for i := range job.Emails {
			if ctx.Err() != nil {
				return
			}
			if !job.Emails[i].Verified {
				job.Emails[i].Verified = verifier.Verify(ctx, job.Emails[i].Addr)
			}
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

func stripHTML(s string) string {
	if s == "" || !strings.ContainsAny(s, "<>") {
		return htmlparser.UnescapeString(s)
	}
	tokenizer := htmlparser.NewTokenizer(strings.NewReader(s))
	var out strings.Builder
	out.Grow(len(s))
	for {
		switch tokenizer.Next() {
		case htmlparser.ErrorToken:
			return htmlparser.UnescapeString(out.String())
		case htmlparser.TextToken:
			out.Write(tokenizer.Text())
		default:
			// skip tags, comments, doctype, etc.
		}
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
