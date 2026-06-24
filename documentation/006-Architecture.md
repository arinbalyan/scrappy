# Architecture

## Overview

scrappy is a modular, Go-native job-board scraper with an engine that fans out across 141 sites concurrently. The architecture follows a clean pipeline:

```
CLI (cmd/scrappy/main.go)
  → Engine (pkg/scrappy/engine.go)
    → Scraper (internal/scraper/scraper.go interface)
      → Model (internal/model/types.go)
    → Email extraction (internal/email/)
    → Quality scoring (internal/quality/)
    → Deduplication (internal/dedup/)
    → Normalization (internal/normalize/)
  → Export (internal/export/)
```

Every layer communicates through the model types. No unnecessary abstractions between packages.

---

## 1. CLI Layer — `cmd/scrappy/main.go`

The binary entry point. Its job is glue: parse flags, load config, build input, call the engine, write output.

**Key flow:**

1. Parse CLI flags into `cliConfig` via cobra.
2. Load `config.toml` (CWD → `~/.scrappy/config.toml`).
3. Load `.env` for API keys.
4. Resolve proxy URLs with TCP-dial health check.
5. Build `model.ScraperInput` — the unified input struct.
6. Call `engine.Scrape(ctx, input)`.
7. Serialize results to the requested format.

Proxy health checking happens at startup:

```go
conn, dialErr := net.DialTimeout("tcp", net.JoinHostPort(host, port), 500*time.Millisecond)
```

Unhealthy proxies are logged and excluded before any scraping begins.

The CLI also supports an interactive wizard (auto-detected on TTY with no args) and two sub-commands: `doctor` (diagnostics) and `setup` (API key configuration).

---

## 2. The `Scraper` Interface — `internal/scraper/scraper.go`

Every site implements this two-method interface:

```go
type Scraper interface {
    Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error)
    SiteName() model.Site
}
```

That's it. No lifecycle hooks, no configuration methods, no middleware interfaces. Each scraper decides how to fetch its site (HTTP GET, API call, RSS feed, Playwright browser) and returns `[]model.JobPost`.

**Site registration** happens at engine initialization in `NewEngine()`:

```go
func NewEngine() *Engine {
    s := []scraper.Scraper{
        indeedscraper.New(nil),
        linkedinscraper.New(nil),
        // ... 100+ scrapers
    }
    m := make(map[model.Site]scraper.Scraper, len(s)+1)
    for _, sc := range s {
        m[sc.SiteName()] = sc
    }
    return &Engine{scrapers: m, siteFailOpen: true}
}
```

The map key is `model.Site` (a string type). Scrapers that need API keys check env vars before running; those with missing credentials produce a warning telemetry entry instead of a failed scrape.

---

## 3. The Model — `internal/model/types.go`

The central data types shared across every layer:

```go
type JobPost struct {
    ID           string
    Title        string
    CompanyName  string
    CompanyURL   string
    JobURL       string
    Location     Location
    IsRemote     bool
    Description  string
    JobType      string
    DatePosted   *time.Time
    Site         string
    Emails       []Email
    Compensation *Compensation
    QualityScore int
    // ... 40+ optional fields
}

type ScraperInput struct {
    Sites           []Site
    SearchTerm      string
    Location        string
    SearchTerms     []string   // multi-term
    Locations       []string   // multi-location
    ResultsWanted   int
    MinScore        int
    MemoryCapMB     int
    Dedup           bool
    // ... per-site overrides, filters, etc.
}
```

The model package also enumerates all 141 sites as typed constants, the `Site` type hierarchy, compensation intervals, job types, and the `Email` struct with verification state.

The `pkg/scrappy/types.go` file re-exports all model types as aliases so external consumers import `pkg/scrappy` instead of the internal package:

```go
type Site = model.Site
type JobPost = model.JobPost
type ScraperInput = model.ScraperInput
// etc.
```

---

## 4. The Engine — `pkg/scrappy/engine.go`

The engine orchestrates everything:

### Concurrency model

```
Global semaphore (channel-based, sized by memory cap or MaxRPS)
  └─ Per-site semaphore (channel-based, sized by --site-rps)
```

- **Global concurrency** scales with `--memory-cap` (3 at 256MB, up to 12 at 1GB+) or `--max-rps`.
- **Per-site semaphores** limit concurrent requests to a single site (default 1-8 based on `--site-rps`).
- Each goroutine also checks `waitForMemoryBudget()` before launching, which blocks while heap exceeds 90% of configured memory cap.

### Scrape loop

1. For each site, launch a goroutine.
2. Each goroutine holds the global sem, then the site sem.
3. For each (search term × location) combination, call `scraper.Scrape()`.
4. Results stream back through a buffered channel.

### Post-processing pipeline (per job)

```go
normalizeJobPost(&jobs[i])            // ensure nil slices
jobs[i].Description = util.StripHTML(...) // strip HTML from text
jobs[i].FetchedAt = &now              // timestamp

// Email extraction (before HTML strip)
htmlEmails := internalemail.ExtractFromHTML(rawHTML)
// Text extraction
found := internalemail.Extract(text)

// Company page enrichment
companyEmails, err := enricher.Enrich(ctx, job.CompanyURL)

// MX verification on every email
verifier.VerifyEmail(ctx, addr)

// Salary normalization
jobs[i].Compensation = normalize.AnnualizeCompensation(...)

// Quality score
jobs[i].QualityScore = quality.Score(&jobs[i])

// Global dedup (by URL)
```

### Memory management

The engine tracks heap usage via `runtime/metrics`. At 80% of `--memory-cap`, it forces GC. Results are eagerly trimmed at 2x `ResultsWanted` to prevent runaway growth.

### Filters (applied after collection)

- `--min-score`: filter by quality score.
- `--email`: keep only jobs with at least one email.
- `--hours-old` / `--since`: age-based filters.
- `--dedup-by-company`: keep one posting per company.

---

## 5. Telemetry — `pkg/scrappy/telemetry.go`

Each site produces a `SiteTelemetry` record:

```go
type SiteTelemetry struct {
    Site              Site
    Attempted         bool
    Success           bool
    Error             string
    FailOpenReason    string    // challenge_detected | rate_limited | access_denied | timeout | unknown
    ResultCount       int
    ChallengeDetected bool
    StatusCodeCount   map[int]int
}
```

The engine also suggests RPS adjustments via `suggestRPS()`: decreases on 429s/captchas, gradually increases on success.

---

## 6. Export — `internal/export/`

After scraping, results are exported to one of four formats: JSONL, CSV, XLSX, or Parquet. Each format has its own file in `internal/export/`. All share a common schema of ~34 columns.

---

## Data flow (end to end)

```
User runs: scrappy --sites linkedin,indeed --search "golang"

1. main.go parses flags → builds ScraperInput
2. main.go TCP-pings proxy URLs, sets SCRAPPY_PROXIES env
3. engine.Scrape() fans out goroutines per site
4. Each goroutine calls linkedin.Scrape() / indeed.Scrape()
5. Scrapers return raw HTML → engine strips HTML
6. Engine runs email extraction (text + HTML)
7. Engine runs MX verification on each email
8. Engine calls quality.Score() per job
9. Engine applies global URL dedup
10. Engine applies filters (min-score, hours-old, etc.)
11. Engine returns []JobPost to main.go
12. main.go serializes to requested format (JSONL/CSV/XLSX/Parquet)
```
