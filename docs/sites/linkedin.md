# LinkedIn

## Current integration
- Search endpoint: `/jobs-guest/jobs/api/seeMoreJobPostings/search`
- Pagination: offset `start` with practical hard cap around `< 1000`
- Optional detail fetch path for description/apply URL enrichment

## Supported knobs
- `search_term` -> `keywords`
- `location` -> `location`
- `distance` -> `distance`
- `is_remote` -> `f_WT=2`
- `job_type` -> `f_JT` (`F/P/I/C/T`)
- `easy_apply` -> `f_AL=true`
- `linkedin_company_ids` -> `f_C`
- `hours_old` -> `f_TPR=r{seconds}`
- `linkedin_fetch_description`
- `linkedin_strategy=rotate` (constraint-aware expansion strategy)
- global resilience knobs: `retries`, `max_rps`, `site_rps`, proxy config

## Constraints and breakpoints
- Guest HTML classes (`base-search-card`, `sr-only`, etc.) are volatile.
- Guest endpoint throttles aggressively (429 behavior).
- Detail page fields can be missing/relocated.
- Result cap requires rotation strategy for broader coverage.

## Debug/update playbook
1. Save failing list/detail HTML fixtures and diff selectors.
2. Validate card extraction count before and after parser changes.
3. Re-check job ID extraction from `href` values.
4. Validate optional detail fields when `linkedin_fetch_description=true`.
5. Tune pacing/proxy/retries for sustained 429 pressure.
6. Re-run `go test ./tests/scraper/linkedin` and then full suite.

## Performance improvements

- **Regex compiled once**: The `reCard` and `reLegacyCard` regex patterns in
  `parseJobCards()` are now compiled at package init time as package-level
  `var` declarations, rather than being recompiled via `regexp.MustCompile`
  on every call to `parseJobCards()`. This reduces GC pressure and improves
  parse throughput, particularly in large scrape runs.
