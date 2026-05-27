# Scraping

`internal/scraper/` -- site-specific scrapers that implement the `Scraper` interface.

## Interface

```go
type Scraper interface {
    Scrape(ctx context.Context, input ScraperInput) ([]JobPost, error)
    SiteName() Site
}
```

The scraper receives a context that carries the per-site rate limiter, so it should never fire requests faster than the token bucket allows.

```go
func (s *IndeedScraper) Scrape(ctx context.Context, input ScraperInput) ([]JobPost, error) {
    for needsMore() {
        if err := s.limiter.Wait(ctx); err != nil {
            return nil, err
        }
        // fetch page, parse, append
    }
    return jobs, nil
}
```

Scrapers use `util.SleepWithContext(ctx, duration)` for context-aware pauses between requests, ensuring timely cancellation when a scrape is terminated.

## All 60+ sites

### High-yield (general boards, largest result sets)

| Site | Page size | Pagination | Hard cap | Notes |
|------|-----------|------------|----------|-------|
| Indeed | 100 | Cursor (`nextCursor`) | None observed | GraphQL; best yield per RPS |
| LinkedIn | 10 | Offset (`start`) | 1,000 | Use `--linkedin-strategy rotate` |
| Google | ~10 | Offset (SERP) | Best-effort | No longer capped at 20; aggressive rate-limiting |
| Adzuna | ~50 | Offset | ~1,000 | Requires `ADZUNA_APP_ID` + `ADZUNA_APP_KEY` |
| Careerjet | ~20 | Offset | ~1,000 | Requires `CAREERJET_AFFID` |
| SimplyHired | ~20 | Offset | Best-effort | Public HTML, US-focused |
| CareerBuilder | ~25 | Offset | ~1,500 | Public HTML |
| Dice | ~20 | Offset | ~1,000 | US tech-focused |
| Monster | ~25 | Offset | ~1,000 | US-focused |
| Reed | ~25 | Offset | ~1,000 | UK-focused |
| InfoJobs | ~20 | Offset | ~1,000 | Requires `INFOJOBS_CLIENT_ID` + `INFOJOBS_CLIENT_SECRET` |

### Medium-yield (remote-first, startup, niche)

| Site | Page size | Pagination | Notes |
|------|-----------|------------|-------|
| RemoteOK | 50 | Offset | JSON in `<script id="job-map">` tag |
| Remotive | ~30 | Offset | JSON API at `remotive.com/api/remote-jobs` |
| RemoteFirstJobs | ~20 | Offset | Remote-only board |
| Jobspresso | ~20 | Offset | Curated remote jobs |
| WorkingNomads | ~20 | Offset | Remote-only digest |
| Himalayas | ~20 | Offset | Remote-focused |
| HiringCafe | ~20 | Offset | Remote startup jobs |
| HuggingFace Jobs | ~20 | Offset | ML/AI focused |
| YC Jobs | ~50 | Offset | Y Combinator companies |
| Wellfound | ~25 | Offset | Startup jobs (formerly AngelList) |
| BuiltIn | ~20 | Offset | Tech-city specific (SF, Boston, NY, Austin) |
| Greenhouse | Varies | Board discovery | Company boards at `boards.greenhouse.io/{co}` |
| GunIO | ~20 | Offset | Remote/startup |
| The Muse | ~20 | API offset | Company profiles + jobs |
| EuroJobs | ~30 | RSS feed | European job board |
| 4 Day Week | ~20 | Offset | 4-day work week companies |
| Web3Career | ~20 | Offset | Web3/crypto jobs |
| Landing.jobs | 50 | Offset | Tech jobs, paginated (max 250) |
| Real Work From Anywhere | ~50 | RSS feed | Remote-only board |

### Niche and regional

| Site | Region / Niche | Pagination |
|------|----------------|------------|
| Internshala | India (internships) | Offset |
| StartupJobs | Central Europe | Offset |
| HasJob | India (startups) | RSS feed |
| VueJobs | Vue.js specific | Offset |
| LaraJobs | Laravel specific | Offset |
| Arbeitnow | Remote/Germany | Offset |
| HackerNews Jobs | `whoishiring` thread | HTML parse |
| CryptocurrencyJobs | Crypto/web3 | Offset |
| AndroidJobs | Android dev | Offset |
| Jobicy | Remote/creative | Offset |
| EcoJobs | Environment/green | RSS feed |
| Golang Jobs | Go-specific | RSS feed |
| DevOpsJobs | DevOps specific | Offset |
| Crunchboard | Tech/startup | Offset |
| CryptoJobsList | Crypto | RSS feed |
| Dribbble | Design jobs | Offset |
| AIJobs | AI/ML | Offset |
| UKVisaJobs | UK visa sponsors | Offset |
| JobsDB | SE Asia | Offset |
| Snagajob | Hourly/retail | Offset |
| Djinni | Ukraine/Europe | Offset |
| HeadHunter | Russia/CIS | Offset |
| MyCareersFuture | Singapore | Offset |
| JobStreet | SE Asia | Offset |
| Jobindex | Denmark | Offset |

### Requires API keys

| Site | Env vars | Sign up |
|------|----------|---------|
| Adzuna | `ADZUNA_APP_ID`, `ADZUNA_APP_KEY` | https://developer.adzuna.com/ |
| Careerjet | `CAREERJET_AFFID` | https://www.careerjet.com/partners/ |
| InfoJobs | `INFOJOBS_CLIENT_ID`, `INFOJOBS_CLIENT_SECRET` | https://developer.infojobs.net/ |
| Findwork | `FINDWORK_API_KEY` | https://findwork.dev/developers/ |
| AuthenticJobs | `AUTHENTICJOBS_API_KEY` | https://authenticjobs.com/api/ |
| Arbeitsagentur | `ARBEITSAGENTUR_API_KEY` | https://rest.arbeitsagentur.de/ |

When a required env var is missing, the engine skips the site with a WARN -- it does not fail the run.

## Browser fallback (anti-bot)

Some sites block plain HTTP requests with JavaScript-based challenges (reCAPTCHA, DataDome, Cloudflare). `scrappy` includes an optional browser fallback that renders these pages in headless Chromium via Playwright:

| Site | Challenge | Browser Strategy |
|------|-----------|------------------|
| Monster | HTTP 403 (DataDome) | Render search page in browser and parse the HTML |

The fallback is **optional** and **silent** -- if Playwright is not installed, the scraper returns the standard blocked error and the site is skipped normally.

### Install browser support

```bash
cd scripts/
npm install
```

The Playwright script at `scripts/fetch-page.mjs` is auto-detected at runtime via `internal/browser/client.go`. It requires Node.js 18+ and is never called when the plain HTTP path succeeds.

## Multi-value cartesian product

Pass comma-separated search terms and locations to generate NxM passes per site:

```bash
# 2 search terms x 2 locations = 4 passes for each site
scrappy --sites indeed --search "AI Engineer,Software Engineer" \
  --location "Remote,New York" --results-wanted 500
```

Each (term, location) pair is an independent scrape. Errors on one pair do not fail others.

See [007-Multi-Value.md](007-Multi-Value.md).

## Performance improvements: regex compilation

Two scrapers had their per-call regex compilation moved to package-level
`var` declarations, avoiding recompilation on every scrape call:

- **LinkedIn**: `reCard` and `reLegacyCard` in `parseJobCards()` are now compiled
  once at package init time.


## Safety improvements

- **RemoteOK**: Now guards against empty API responses. The API returns an array
  where index 0 is metadata; if the response has fewer than 2 elements (missing
  the metadata row or no jobs), the scraper returns an empty result set instead
  of panicking with an index-out-of-bounds error. Context cancellation is also
  checked between page fetches.
- **Adzuna**: Now enforces 500ms rate limiting between API page requests and
  limits pages to a maximum of 20 (down from 100). HTTP response bodies are
  properly deferred-closed on all code paths (the previous code could leave
  bodies unclosed if an error occurred mid-function).

## Per-site concurrency defaults

| Site | Max concurrent | Max RPS |
|------|---------------|---------|
| LinkedIn | 1-2 | 1 req/3s |
| Indeed | 10 | 3 req/s |
| Google | 2 | 1 req/2s |
| ZipRecruiter | 4 | 2 req/s |
| Adzuna | 4 | 2 req/s |
| Careerjet | 4 | 2 req/s |
| Wellfound / RemoteOK / Remotive | 8 | 5 req/s |
| All others | 4-8 | 3 req/s (default) |

Per-semaphore limits are configurable via `--site-rps`:

```bash
scrappy --site-rps linkedin:1,indeed:10 --sites linkedin,indeed --search "engineer"
```

Global concurrency is governed by a semaphore sized according to `--max-rps` or `--memory-cap`. See [016-Memory-Management.md](016-Memory-Management.md).

## Fail-open behavior

A site error (429, 5xx, CAPTCHA, timeout) does **not** abort the entire run. The engine:

1. Records the error in `SiteTelemetry`
2. Sets `FailOpenReason` (challenge_detected, rate_limited, access_denied, timeout, unknown)
3. Continues to the next (term, location) pair or site
4. Reports partial results for that site if any were collected

`SuggestRPS` automatically ratchets down after 429/rate-limit errors and ratchets up on success.

## Rate limiting via `--max-rps` and `--site-rps`

In addition to per-site defaults, two CLI flags provide rate control:

- `--max-rps`: Global maximum requests per second, clamped between 2 and 16. Overrides the default concurrency of 8.
- `--site-rps`: Per-site overrides in `site:rps` format (e.g. `linkedin:1,indeed:10`). These replace the per-site defaults.

```bash
scrappy --max-rps 10 --site-rps linkedin:1,indeed:5 \
  --sites linkedin,indeed --search "engineer"
```

## Telemetry

After a run, `RunTelemetry` contains per-site stats:

```go
type SiteTelemetry struct {
    Site              Site
    Attempted         bool
    Success           bool
    Error             string
    FailOpenReason    string
    ResultCount       int
    EmptyPageRate     float64
    CaptchaRate       float64
    CursorStalls      int
    StatusCodeCount   map[int]int
    ChallengeDetected bool
}
```

Access via `engine.Telemetry()` in library mode, or enable `--log-level DEBUG` in CLI mode.

## Country pass-through for Indeed

The `country` field in config.yaml (and the `SCRAPPY_INDEED_CO` env var) sets the `indeed-co` header and search host for country-specific results. Supported values include `germany`, `uk`, `india`, `australia`, `canada`, and others. This is passed through to the Indeed scraper via `ScraperInput.Country`.

See [008-Configuration.md](008-Configuration.md) for usage examples.
