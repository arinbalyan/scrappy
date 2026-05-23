# Overview

`scrappy` is a bulk-first job-board scraper written in Go. It fans out across 65+ sites concurrently, processes thousands of postings per run, and ships results to disk without materializing everything in memory.

```bash
scrappy --sites linkedin,indeed,remoteok --search "golang" \
  --location "Remote" --results-wanted 500 \
  --format parquet --out /data/jobs.parquet
```

## Why Go

| Concern | Python libraries | scrappy (Go) |
|---|---|---|
| Concurrency | ThreadPoolExecutor (OS-thread bound) | goroutines + errgroup (lightweight) |
| Memory | High (per-thread stack, GIL overhead) | Low (goroutine stack ~2 KB), memory-cap aware |
| Binary | Requires Python runtime | Single static binary (~10 MB) |
| Proxy rotation | Round-robin, no health checks | SOCKS5 pool with pre-flight probes |
| Email enrichment | Bare regex, no validation | Extract → normalize → MX lookup → company-page follow-up |
| Deduplication | Per-DataFrame only | Cross-site `sync.Map` dedup + company dedup |
| Exports | pandas DataFrame only | CSV, JSONL, XLSX, Parquet |

## Feature highlights

- **65+ scrapers** — built-in support for LinkedIn, Indeed, Glassdoor, Google, ZipRecruiter, and 60+ niche/regional boards
- **Bulk-first design** — fan-out across all sites concurrently, aggregate thousands of postings in a single run
- **Email enrichment** — regex extraction + MX DNS validation + company-page follow-up (catches ~60–80% of postings with a contact address)
- **Deterministic quality score** — 0–100 per posting based on salary presence, apply method, email match, freshness, description length, agency check
- **Multi-value cartesian product** — comma-separated search terms × locations produce N×M scrape passes per site (errors on one don't fail others)
- **Memory-aware concurrency** — cap total memory with `--memory-cap 512MB`; scales scraper count from 3 (≤256 MB) to 12 (>1 GB) with periodic heap monitoring
- **Config auto-detect** — loads `config.yaml` from current directory or `~/.scrappy/config.yaml`; `.env` files loaded from same locations
- **Output formats** — JSONL (default), CSV, XLSX, Parquet
- **Go library** — import `github.com/arinbalyan/scrappy/pkg/scrappy` for programmatic use

## Quick examples

```bash
# Interactive wizard (no flags)
scrappy

# Scrape a few sites with filters
scrappy --sites glassdoor,ziprecruiter --search "rust developer" \
  --location "Remote" --is-remote --job-type fulltime --results-wanted 200

# Multi-value: 2 terms × 2 locations = 4 passes per site
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
├── cmd/
│   └── scrappy/            # CLI entrypoint (cobra)
├── pkg/
│   └── scrappy/            # Public Go library (Engine, ScraperInput)
├── internal/
│   ├── model/              # JobPost, ScraperInput, Site, Country, Email
│   ├── scraper/            # 66 site-specific scraper packages
│   │   ├── scraper.go      # Scraper interface
│   │   ├── linkedin/       # LinkedIn scraper
│   │   ├── indeed/         # Indeed scraper
│   │   ├── glassdoor/      # Glassdoor scraper
│   │   ├── google/         # Google Jobs scraper
│   │   ├── remoteok/       # RemoteOK scraper
│   │   └── ...             # 60+ more site packages
│   ├── rate/               # Per-site token-bucket rate limiter
│   ├── proxy/              # SOCKS5/HTTP proxy pool + health probes
│   ├── email/              # Email extraction, MX validation, company-page enrich
│   ├── dedup/              # Thread-safe URL + company deduplication
│   ├── quality/            # Deterministic quality score (0–100)
│   ├── export/             # Writers: jsonl, csv, xlsx, parquet
│   ├── normalize/          # Salary/title normalization (placeholder)
│   └── util/               # HTTP client, logging, retry helpers
├── tests/
│   └── scraper/            # Contract tests per scraper
├── config.yaml             # Per-site search/location defaults
├── go.mod                  # Module: github.com/arinbalyan/scrappy
└── .github/workflows/
    └── ci.yml              # Go 1.26 build + test + vet + gitleaks
```

## Dependencies

scrappy relies on Go standard library plus four direct dependencies:

| Package | Purpose |
|---|---|
| `github.com/spf13/cobra` | CLI flag parsing |
| `gopkg.in/yaml.v3` | Config file loading |
| `golang.org/x/time/rate` | Per-site token-bucket rate limiter |
| `github.com/xuri/excelize/v2` | XLSX export |
| `github.com/xitongsys/parquet-go` | Parquet export |

## Sites supported

65+ job boards across general, remote, startup, niche, and regional categories. Run `scrappy --help` to see the full list. All implement the same `Scraper` interface:

```go
type Scraper interface {
    Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error)
    SiteName() model.Site
}
```

Five sites require API keys (set in `~/.scrappy/.env` or environment): Adzuna, Careerjet, InfoJobs, Findwork, and Arbeitsagentur.
