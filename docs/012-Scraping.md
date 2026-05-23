# Scraping

`internal/scraper/` — site-specific scrapers that implement the `Scraper` interface.

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

## All 65+ sites

### High-yield (general boards, largest result sets)

| Site | Page size | Pagination | Hard cap | Notes |
|---|---|---|---|---|
| Indeed | 100 | Cursor (`nextCursor`) | None observed | GraphQL; best yield per RPS |
| LinkedIn | 10 | Offset (`start`) | 1,000 | Use `--linkedin-strategy rotate` |
| Google | ~10 | Offset (SERP) | Best-effort | Aggressive rate-limiting |
| Glassdoor | ~30 | Cursor | ~1,000 | Dates rounded to next day |
| ZipRecruiter | ~20 | Cursor | ~1,000 | US/Canada only |
| Adzuna | ~50 | Offset | ~1,000 | Requires `ADZUNA_APP_ID` + `ADZUNA_APP_KEY` |
| Careerjet | ~20 | Offset | ~1,000 | Requires `CAREERJET_AFFID` |
| SimplyHired | ~20 | Offset | Best-effort | Public HTML, US-focused |
| CareerBuilder | ~25 | Offset | ~1,500 | Public HTML |
| Jooble | ~20 | Offset | ~1,000 | Aggregation engine |
| Dice | ~20 | Offset | ~1,000 | US tech-focused |
| Monster | ~25 | Offset | ~1,000 | US-focused |
| Reed | ~25 | Offset | ~1,000 | UK-focused |
| StepStone | ~20 | Offset | ~1,000 | Germany/Austria |
| InfoJobs | ~20 | Offset | ~1,000 | Requires `INFOJOBS_CLIENT_ID` + `INFOJOBS_CLIENT_SECRET` |

### Medium-yield (remote-first, startup, niche)

| Site | Page size | Pagination | Notes |
|---|---|---|---|
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
| Academic Careers | ~20 | RSS feed | Academia/research positions |
| Web3Career | ~20 | Offset | Web3/crypto jobs |
| Upwork | ~20 | Offset | Freelance/contract |

### Niche & regional

| Site | Region / Niche | Pagination |
|---|---|---|
| Bayt | Middle East | Offset |
| BDJobs | Bangladesh | Offset |
| Naukri | India | Offset |
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
| DevOpsJobs | DevOps specific | Offset |
| Crunchboard | Tech/startup | Offset |
| IOSDevJobs | iOS dev | Offset |
| SwissDevJobs | Switzerland | Offset |
| CryptoJobsList | Crypto | RSS feed |
| DevITJobs | Germany/Dev | Offset |
| Dribbble | Design jobs | Offset |
| AIJobs | AI/ML | Offset |
| Wuzzuf | Egypt/MENA | Offset |
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
|---|---|---|
| Adzuna | `ADZUNA_APP_ID`, `ADZUNA_APP_KEY` | https://developer.adzuna.com/ |
| Careerjet | `CAREERJET_AFFID` | https://www.careerjet.com/partners/ |
| InfoJobs | `INFOJOBS_CLIENT_ID`, `INFOJOBS_CLIENT_SECRET` | https://developer.infojobs.net/ |
| Findwork | `FINDWORK_API_KEY` | https://findwork.dev/developers/ |
| Arbeitsagentur | `ARBEITSAGENTUR_API_KEY` | https://rest.arbeitsagentur.de/ |

When a required env var is missing, the engine skips the site with a WARN — it does not fail the run.

## Multi-value cartesian product

Pass comma-separated search terms and locations to generate N×M passes per site:

```bash
# 2 search terms × 2 locations = 4 passes for each site
scrappy --sites indeed --search "AI Engineer,Software Engineer" \
  --location "Remote,New York" --results-wanted 500
```

Each (term, location) pair is an independent scrape. Errors on one pair don't fail others.

## Per-site concurrency defaults

| Site | Max concurrent | Max RPS |
|---|---|---|
| LinkedIn | 1–2 | 1 req/3s |
| Indeed | 10 | 3 req/s |
| Glassdoor | 4 | 2 req/s |
| Google | 2 | 1 req/2s |
| ZipRecruiter | 4 | 2 req/s |
| Adzuna | 4 | 2 req/s |
| Careerjet | 4 | 2 req/s |
| Wellfound / RemoteOK / Remotive | 8 | 5 req/s |
| All others | 4–8 | 3 req/s (default) |

Per-semaphore limits are configurable via `--site-rps`:

```bash
scrappy --site-rps linkedin:1,indeed:10 --sites linkedin,indeed --search "engineer"
```

## Fail-open behavior

A site error (429, 5xx, CAPTCHA, timeout) does **not** abort the entire run. The engine:

1. Records the error in `SiteTelemetry`
2. Sets `FailOpenReason` (challenge_detected, rate_limited, access_denied, timeout, unknown)
3. Continues to the next (term, location) pair or site
4. Reports partial results for that site if any were collected

`SuggestRPS` automatically ratchets down after 429/rate-limit errors and ratchets up on success.

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

## Rate limiting

Each site's HTTP session respects a `rate.Limiter` injected via context. The limiter is a `golang.org/x/time/rate.Limiter` (token bucket) keyed by hostname, configurable via `--site-rps`. Global concurrency is governed by a semaphore sized according to `--max-rps` or `--memory-cap`.
