# Architecture Overview

`scrappy` is a bulk-first job-board scraper written in Go. It fans out across 62+ sites concurrently, processes thousands of postings per run, and ships results to disk without materializing everything in memory.

```bash
scrappy --sites linkedin,indeed,remoteok --search "golang" \
  --location "Remote" --results-wanted 500 \
  --format parquet --out /data/jobs.parquet
```

## Why Go

| Concern | Python libraries | scrappy (Go) |
|---------|-----------------|--------------|
| Concurrency | ThreadPoolExecutor (OS-thread bound) | goroutines + errgroup (lightweight) |
| Memory | High (per-thread stack, GIL overhead) | Low (goroutine stack ~2 KB), memory-cap aware |
| Binary | Requires Python runtime | Single static binary (~10 MB) |
| Proxy rotation | Round-robin, no health checks | SOCKS5 pool with TCP-dial + HTTP pre-flight probes |
| Email enrichment | Bare regex, no validation | Extract, normalize, MX lookup, company-page follow-up |
| Deduplication | Per-DataFrame only | Cross-site URL dedup + company dedup (thread-safe) |
| Exports | pandas DataFrame only | CSV, JSONL, XLSX, Parquet |
| Race safety | GIL-reliant | Explicit race detection via `-race` flag, concurrent maps, atomics |

## Feature highlights

- **62+ scrapers** -- built-in support for LinkedIn, Indeed, Glassdoor, Google, ZipRecruiter, and 57+ niche/regional boards
- **Bulk-first design** -- fan-out across all sites concurrently, aggregate thousands of postings in a single run
- **Email enrichment** -- regex extraction + MX DNS validation + company-page follow-up (catches ~60-80% of postings with a contact address)
- **Deterministic quality score** -- 0-100 per posting based on salary presence, apply method, email-domain/company-domain match, freshness, description length, agency check
- **Multi-value cartesian product** -- comma-separated search terms x locations produce NxM scrape passes per site (errors on one do not fail others)
- **Memory-aware concurrency** -- cap total memory with `--memory-cap 512MB`; scales scraper count from 3 (up to 256 MB) to 12 (more than 1 GB) with periodic heap monitoring
- **Rate limiting** -- per-site token-bucket via `golang.org/x/time/rate`, global RPS cap via `--max-rps`, per-site overrides via `--site-rps`
- **Config auto-detect** -- loads `config.yaml` from current directory or `~/.scrappy/config.yaml`; `.env` files loaded from same locations
- **Output formats** -- JSONL (default), CSV, XLSX, Parquet
- **Go library** -- import `github.com/arinbalyan/scrappy/pkg/scrappy` for programmatic use

## Quick examples

```bash
# Interactive wizard (no flags)
scrappy

# Scrape a few sites with filters
scrappy --sites glassdoor,ziprecruiter --search "rust developer" \
  --location "Remote" --is-remote --job-type fulltime --results-wanted 200

# Multi-value: 2 terms x 2 locations = 4 passes per site
scrappy --sites indeed --search "AI Engineer,Software Engineer" \
  --location "Remote,New York" --results-wanted 500

# Memory-constrained run
scrappy --sites linkedin,indeed --search "golang" --location "Remote" \
  --memory-cap 512MB --results-wanted 200 --format jsonl

# Non-interactive for cron/CI
scrappy --sites indeed --search "devops" --location "Remote" \
  --results-wanted 100 --format csv --out /data/jobs.csv \
  --non-interactive
```

## Repository layout

```
scrappy/
  cmd/
    scrappy/            # CLI entrypoint (cobra)
  pkg/
    scrappy/            # Public Go library (Engine, ScraperInput)
  internal/
    model/              # JobPost, ScraperInput, Site, Country, Email
    scraper/            # 65 site-specific scraper packages
      scraper.go        # Scraper interface
      linkedin/         # LinkedIn scraper
      indeed/           # Indeed scraper
      glassdoor/        # Glassdoor scraper
      google/           # Google Jobs scraper
      remoteok/         # RemoteOK scraper
      ...               # 60+ more site packages
    rate/               # Per-site token-bucket rate limiter
    proxy/              # SOCKS5/HTTP proxy pool + health probes
    email/              # Email extraction, MX validation, company-page enrich
    dedup/              # Thread-safe URL + company deduplication
    quality/            # Deterministic quality score (0-100)
    export/             # Writers: jsonl, csv, xlsx, parquet
    util/               # HTTP client, logging, retry helpers, context-aware sleep
  tests/
    scraper/            # Contract tests per scraper
  config.yaml           # Per-site search/location defaults
  .env.example          # Environment variable template
  Dockerfile            # Multi-stage build ~10 MB
  docker-compose.yml    # Scheduled bulk runs
  go.mod                # Module: github.com/arinbalyan/scrappy
  .github/workflows/
    ci.yml              # Go 1.26 build + test + vet + gitleaks
```

## Dependencies

scrappy relies on Go standard library plus these direct dependencies:

| Package | Purpose |
|---------|---------|
| `github.com/spf13/cobra` | CLI flag parsing |
| `gopkg.in/yaml.v3` | Config file loading |
| `golang.org/x/time/rate` | Per-site token-bucket rate limiter |
| `github.com/xuri/excelize/v2` | XLSX export |
| `github.com/xitongsys/parquet-go` | Parquet export |

All HTTP, JSON, CSV, and concurrency primitives use Go standard library.

## Sites supported

62+ job boards across general, remote, startup, niche, and regional categories. Run `scrappy --help` to see the full list. All implement the same `Scraper` interface:

```go
type Scraper interface {
    Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error)
    SiteName() model.Site
}
```

Five sites require API keys (set in `.env` files or environment): Adzuna, Careerjet, InfoJobs, Findwork, and Arbeitsagentur.

## Data pipeline

The full pipeline from CLI invocation to output:

```
CLI flags + config.yaml + .env
  |
  v
ScraperInput construction (search, sites, filters, rates, memory cap)
  |
  v
Per-site rate.Limiter (token bucket, configurable RPS)
Per-site goroutine group (errgroup with semaphore)
  |
  v
Each scraper: HTTP fetch -> parse HTML/JSON -> []JobPost
  |  - Context-aware sleep via util.SleepWithContext()
  |  - Email extraction per posting (regex + MX lookup)
  |  - Domain population from emails for quality scoring
  v
Cross-site dedup: URL dedup + optional company dedup
  |
  v
Quality scoring: deterministic 0-100 per posting (with concurrent maps/atomics)
  |
  v
MinScore filter, EmailsOnly filter, ResultsWanted truncation
  |
  v
Export: JSONL / CSV / XLSX / Parquet (streaming, no full materialization)
```

## Key design decisions

### Race safety

The codebase is tested with `go test -race ./...` to catch data races. Quality scoring uses concurrent maps with atomic counters for thread-safe aggregation. All dedup sets use `sync.Mutex` for safe concurrent access.

### Context-aware sleep

Instead of `time.Sleep()`, scrapers use `util.SleepWithContext(ctx, duration)` which respects context cancellation and deadlines. This prevents goroutine leaks when a scrape is cancelled mid-wait.

### Domain population from emails

When emails are extracted from a job posting, the company domain is derived from the email domain and stored on the `JobPost.Domain` field. This domain is used in quality scoring (email-domain/company-domain match worth +15 points) and deduplication heuristics.

### Fail-open design

A site error (429, 5xx, CAPTCHA, timeout) does not abort the entire run. The engine records the error in telemetry and continues to the next site. See [012-Scraping.md](012-Scraping.md) for details.
