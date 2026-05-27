# 4DayWeek

## Current integration
- Parser order:
  1. JSON API at `https://4dayweek.io/api/jobs` — paginated with `page` query param (max 5 pages)
- Extracted fields: id, title, company, slug, work arrangement (remote/hybrid/onsite), location, salary (lower/upper/currency/period), schedule type, category, level, company logo, work-life score

## Supported knobs
- `results_wanted`
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)

## Constraints and breakpoints
- `has_more` flag in the API response controls pagination.
- Salary is stored in cents with `salary_lower`/`salary_upper` (divided by 100 on output).
- Schedule type maps to `ApplyMethod` (e.g. "4-day week (100% pay)", "Flex Fridays").

## Debug/update playbook
1. Check the API response at `https://4dayweek.io/api/jobs` for field changes.
2. Verify salary conversion (cents -> float division by 100).
3. Update `maxPages` constant if the API adds more pages.
4. Re-run `go test ./tests/scraper/4dayweek` and full suite.
