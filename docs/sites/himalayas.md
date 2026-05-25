# Himalayas

## Current integration
- Endpoint: `https://himalayas.app/jobs/api`
- Response shape: JSON object with `jobs` array, `totalCount`, `offset`, `limit`.
- Pagination: offset-based (`limit`, `offset` params), capped at 10 pages (`maxPages=10`) with page size 20.

## Supported knobs
- `results_wanted`
- global resilience knobs: `retries`, `max_rps`, `site_rps`, proxy config

## Constraints and breakpoints
- Remote-only board — `IsRemote` is hardcoded to `true`.
- Offset-based pagination with a hard cap of 10 pages (200 results maximum).
- `pubDate` is a Unix timestamp in seconds.
- `locationRestrictions` is an array; only the first entry is used for the job location.
- `seniority` is an array; joined as comma-separated string for both `Seniority` and `JobLevel`.
- Compensation in `minSalary` / `maxSalary` fields (USD default currency).
- `applicationLink` is the direct job URL; falls back to `https://himalayas.app/jobs`.

## Debug/update playbook
1. Confirm API response still uses `jobs` / `totalCount` / `offset` / `limit` envelope.
2. Verify `limit` and `offset` pagination parameters correctly advance pages.
3. Validate `pubDate` (Unix timestamp) parsing.
4. Check `locationRestrictions` array structure (first-entry contract).
5. Re-run `go test ./tests/scraper/himalayas` and then full suite.
