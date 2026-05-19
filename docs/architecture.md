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

- `internal/` for all non-exported logic. `pkg/` for anything `jobhunter` or external consumers need.
- Site scrapers are individual packages (`internal/scraper/indeed/`) so they can be unit-tested in isolation.
- The email, dedup, and quality packages are standalone — `jobhunter` can also use them.
