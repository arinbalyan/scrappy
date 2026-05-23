# Architecture

## Pipeline

```
scrappy scrape (CLI or Go library)
  |
  +-- [rate/]     Per-site token-bucket rate limiter
  +-- [proxy/]    Proxy pool + health probe + SOCKS5
  +-- [scraper/*] Site scrapers (concurrent, bounded by per-site semaphores)
  |     |
  |     +-- [email/]       Email extractor + MX validator
  |     +-- [normalize/]   Salary normalization, title normalization
  +-- [dedup/]    URL deduplicator + company dedup (sync.Map, cross-site)
  +-- [quality/]  Deterministic score 0–100
  +-- [export/]   Writer → csv / xlsx / jsonl / parquet
```

## Data flow

1. CLI or library call creates a `ScraperInput` with site list, search term, filters.
2. Per-site `rate.Limiter` enforces RPS cap. Each site's goroutine group hits the site's HTTP endpoint(s).
3. Raw JSON / HTML responses are normalized to `[]JobPost` by the site-specific scraper.
4. Email extractor runs on each posting's description text. Company page follow-up runs concurrently per distinct domain.
5. Salary normalization converts all compensation to annual USD.
6. Deduplicator removes cross-site duplicate URLs and by-company duplicates.
7. Quality scorer adds a 0–100 score to each posting.
8. Exporter streams filtered results to the chosen format via Go channels — no full in-memory materialization required for Parquet/JSONL.

## Packaging decisions

- `internal/` for all non-exported logic. `pkg/` for anything external consumers need.
- Site scrapers are individual packages (`internal/scraper/indeed/`) so they can be unit-tested in isolation.
- The email, dedup, and quality packages are standalone — external consumers can also use them.

## Multi-value cartesian product

When `SearchTerms` and `Locations` contain multiple values (from comma-separated CLI flags or multi-entry config.yaml), the engine generates a cartesian product before scraping each site:

```
terms  = ["AI Engineer", "Software Developer"]
locs   = ["Remote", "New York"]
       ↓
passes = (AI Engineer × Remote), (AI Engineer × New York),
         (Software Developer × Remote), (Software Developer × New York)
```

Per-site search/location overrides in `config.yaml` replace the global values for that specific site. Errors on one (term, loc) combo are logged via fail-open telemetry but do not abort other combos or sites. Results from all successful passes are aggregated before post-processing.

Implementation in `pkg/scrappy/engine.go`:

```go
for _, term := range terms {
    for _, loc := range locs {
        siteInput.SearchTerm = term
        siteInput.Location = loc
        jobs, err := sc.Scrape(ctx, siteInput)
        // errors do not break the loop
    }
}
```

## Memory cap integration

The `--memory-cap` flag (or `memory_cap` in config.yaml) controls two things:

1. **Global concurrency semaphore** — limits how many sites run simultaneously. Scaled via `globalConcurrency()`:

| MemoryCapMB | Concurrent scrapers |
|---|---|
| ≤256 | 3 |
| ≤512 | 5 |
| ≤1024 | 8 |
| >1024 | 12 |
| 0 (unlimited) | 8 or input.MaxRPS |

2. **Heap monitor goroutine** — when `MemoryCapMB > 0`, a background goroutine reads `runtime.MemStats` every 10 seconds. If heap allocation exceeds 80% of the cap, it logs a warning with current alloc, cap, and GC cycle count. The goroutine stops when the scrape context is cancelled.

```go
if input.MemoryCapMB > 0 {
    memThreshold := uint64(input.MemoryCapMB) * 1024 * 1024 * 8 / 10 // 80%
    go func() {
        ticker := time.NewTicker(10 * time.Second)
        for {
            select {
            case <-ctx.Done():
                return
            case <-ticker.C:
            }
            var m runtime.MemStats
            runtime.ReadMemStats(&m)
            if m.Alloc > memThreshold {
                util.Warn("memory_pressure", ...)
            }
        }
    }()
}
```

## Config loading pipeline

scrappy resolves configuration in this priority order (highest wins):

```
1. CLI flags                          (--search, --sites, --memory-cap, etc.)
2. config.yaml in current directory   (./config.yaml)
3. ~/.scrappy/config.yaml             (user-wide defaults)
4. Built-in defaults                  (empty string, 0 results-wanted, etc.)
```

For each setting, the first non-empty value in this chain is used. `.env` files are loaded from the same two directories (beside `config.yaml`) and populate environment variables — but existing env vars are never overwritten.

```go
func defaultConfigPath() string {
    // 1. ./config.yaml
    if _, err := os.Stat("config.yaml"); err == nil {
        return "config.yaml"
    }
    // 2. ~/.scrappy/config.yaml
    userCfg := filepath.Join(homeDir(), ".scrappy", "config.yaml")
    if _, err := os.Stat(userCfg); err == nil {
        return userCfg
    }
    return "config.yaml"
}
```

Site-specific settings (`sites.indeed.search`, etc.) override global search/location for that site at scrape time via `SiteSearch` and `SiteLocations` maps on `ScraperInput`.

## Post-processing pipeline

After scraping, each `JobPost` passes through a deterministic pipeline before export:

```
raw JobPost from scraper
  │
  ├─ 1. stripHTML (description + company_description)
  ├─ 2. set Site + FetchedAt
  ├─ 3. enrichJobEmails (regex extract + MX verify + dedup)
  ├─ 4. quality.Score (compute 0–100)
  ├─ 5. custom hooks (engine.RegisterHook)
  ├─ 6. dedupWithinSite (remove duplicate URLs per site)
  │
  ├─ cross-site dedup.DedupFilters (URL dedup + optional company dedup)
  ├─ MinScore filter  (quality score threshold)
  ├─ EmailsOnly filter (discard jobs with zero emails)
  ├─ sort by ID
  └─ truncate to ResultsWanted
```

The pipeline runs per-job (steps 1–5) in a loop over results, then runs cross-site filters (steps 6+) on the aggregated list. This design keeps memory O(n) in the total result count.

## Key packages

| Package | Responsibility |
|---|---|
| `cmd/scrappy/` | CLI entrypoint: cobra flags, config loading, interactive wizard, output dispatch |
| `pkg/scrappy/` | Public Engine: orchestrates scrape runs, post-processing pipelines, telemetry |
| `internal/model/` | Core types: `JobPost`, `ScraperInput`, `Site`, `Country`, `Email`, `Compensation` |
| `internal/scraper/` | `Scraper` interface + 66 site-specific scraper packages |
| `internal/rate/` | Per-site token-bucket rate limiters (`golang.org/x/time/rate`) |
| `internal/proxy/` | Proxy pool: SOCKS5/HTTP URL parsing, health checking (HEAD probes), rotate/stickiness |
| `internal/email/` | Email pipeline: regex extraction, RFC validation, MX DNS lookup, company-page enrichment |
| `internal/dedup/` | Thread-safe `Set` for URL and company-name deduplication |
| `internal/quality/` | Deterministic quality score (salary +30, apply method +20, email match +15, freshness +15, description length +10, non-agency +10) |
| `internal/export/` | Output writers: `WriteJSONL`, `WriteCSV`, `WriteXLSX`, `WriteParquet` |
| `internal/util/` | Shared HTTP client (retry, UA rotation, proxy dialing), structured logging (`Info`/`Warn`/`Error`/`Debug`/`APIMiss`), response helpers |
