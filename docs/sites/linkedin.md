# LinkedIn

## Current integration
- Search endpoint: `/jobs-guest/jobs/api/seeMoreJobPostings/search`
- Pagination: offset `start`, hard cap around `< 1000`
- Key params from input:
  - `search_term` -> `keywords`
  - `location` -> `location`
  - `distance` -> `distance`
  - `is_remote` -> `f_WT=2`
  - `job_type` -> `f_JT` (`F/P/I/C/T`)
  - `easy_apply` -> `f_AL=true`
  - `linkedin_company_ids` -> `f_C`
  - `hours_old` -> `f_TPR=r{seconds}`

## Fragile points / likely breakpoints
- Guest HTML class names (`base-search-card`, `sr-only`, etc.)
- Job detail page structure (description/apply URL blocks)
- Anti-bot/429 behavior and guest endpoint throttling

## Debug checklist when it breaks
1. Save failing HTML to fixture and inspect class/attribute changes.
2. Validate card extraction count before/after parser edits.
3. Re-check `href` job ID capture logic.
4. Re-check optional detail fields when `linkedin_fetch_description=true`.
5. Watch 429s and tune request pacing/retries/proxy.
