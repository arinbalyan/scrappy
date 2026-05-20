# Indeed

## Current integration
- Endpoint: `https://apis.indeed.com/graphql`
- Query shape: `jobSearch(what, location, limit=100, cursor, filters)`
- Pagination: cursor-based via `pageInfo.nextCursor`

## Supported knobs
- `search_term` -> `what`
- `location` + `distance` -> `location.where/radius`
- `hours_old` -> `filters.date.start = "{hours}h"`
- `easy_apply` -> `filters.keyword(field=indeedApplyScope, keys=[DESKTOP])`
- `job_type` / `is_remote` -> composite `attributes` filter keys
- global resilience knobs: `retries`, `max_rps`, `site_rps`, proxy config

## Constraints and breakpoints
- GraphQL response field paths can drift (`data.jobSearch.results[].job.*`).
- Compensation nesting varies (`estimated`, `baseSalary`, `range`).
- Employer detail nesting can change (`employer.dossier.employerDetails`).
- Attribute keys for remote/job type are brittle and country-dependent.

## Debug/update playbook
1. Capture status code + response body for non-2xx and schema mismatches.
2. Validate path `data.jobSearch.results` and cursor progression (`nextCursor`).
3. Re-validate filter key constants against live payload.
4. Re-check compensation interval/unit mapping (`YEAR/MONTH/WEEK/DAY/HOUR`).
5. Re-run `go test ./tests/scraper/indeed` and then full suite.
