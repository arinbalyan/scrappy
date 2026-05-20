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

## Site-specific notes

| Site | Page size | Pagination | Hard cap | Notes |
|---|---|---|---|---|
| Indeed | 100 jobs/page | Cursor (`nextCursor`) | None observed | GraphQL; best yield per RPS |
| LinkedIn | 10 jobs/page | Offset (`start`) | 1,000 results | Use `--linkedin-strategy rotate` |
| Glassdoor | ~30 jobs/page | Cursor | ~1,000 | Glassdoor rounds dates to next day |
| ZipRecruiter | ~20 jobs/page | Cursor | ~1,000 | US/Canada only |
| Google | ~10 jobs/page | Offset (SERP) | Best-effort | Aggressive rate-limiting |
| Wellfound | ~25 jobs/page | Offset | None known | Public HTML |
| RemoteOK | 50 jobs/page | Offset | None known | JSON in page `<script>` tag |
| Remotive | ~30 jobs/page | Offset | None known | JSON API embedded |

## Per-site concurrency defaults

| Site | Max concurrent | Max RPS |
|---|---|---|
| LinkedIn | 1–2 goroutines | 1 req/3s |
| Indeed | 10 goroutines | 3 req/s |
| Glassdoor | 4 goroutines | 2 req/s |
| Google | 2 goroutines | 1 req/2s |
| ZipRecruiter | 4 goroutines | 2 req/s |
| Wellfound / RemoteOK / Remotive | 8 goroutines | 5 req/s |
| Workable Jobs | 4 goroutines | 2 req/s |
| MyWorkdayJobs | 4 goroutines | 2 req/s |

## New native ATS sources

### workable_jobs
- Site key: `workable_jobs`
- Discovery inputs: `--workable-seeds` or `SCRAPPY_WORKABLE_SEEDS` (comma-separated). CLI overrides env.
- Seed examples: `spotify,datadog` or `https://apply.workable.com/spotify/`.
- Filtering: role filter uses flexible contains + synonym matching against title/description.
- Extracted fields: title, company, url, description, department, job type, posted date, location, remote flag.

### myworkdayjobs
- Site key: `myworkdayjobs`
- Discovery inputs: `--workday-seeds` or `SCRAPPY_WORKDAY_SEEDS` (comma-separated). CLI overrides env.
- Seed format: Workday CXS endpoints (must include `/wday/cxs/`).
- Filtering: role filter uses flexible contains + synonym matching against title/description.
- Extracted fields: title, company, url, description, posted date, location, remote flag, inferred experience range.

## Development process (this feature)
1. Added new site enums and engine wiring.
2. Implemented native Workable and Workday scrapers with guarded body reads.
3. Added role synonym filtering and seed normalization.
4. Added strict parser/filter tests for both scrapers.
5. Added constraints warnings for missing discovery seeds.

## Rate limiting

Each site's HTTP session respects a `rate.Limiter` that is injected via context. The limiter is a `golang.org/x/time/rate.Limiter` (token bucket) keyed by hostname, configurable via `--site-rps`.
