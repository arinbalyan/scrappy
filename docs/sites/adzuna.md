# Adzuna

## Current integration
- Parser order:
  1. JSON API at `https://api.adzuna.com/v1/api/jobs/{country}/search/{page}`
- Extracted fields: id, title, company, redirect_url, location, salary_min, salary_max, created, contract_time, description

## Supported knobs
- `results_wanted`
- `search_term` — server-side via `what` param
- `location` — server-side via `where` param
- `hours_old` — server-side via `max_days_old` param
- `country` — mapped to Adzuna 2-letter country codes (us, ca, gb, de, fr, in, au)
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)

## Constraints and breakpoints
- **Requires `ADZUNA_APP_ID` and `ADZUNA_APP_KEY`** environment variables.
- Salary with `salary_is_predicted=1` is excluded to avoid estimated values.
- Pagination is page-based with a hard cap of 20 pages.
- Page size is adaptive: defaults to 50 or `results_wanted` (whichever is smaller).
- Rate-limited to one request per 500ms by the scraper.
- Contract time maps to: `full_time` → fulltime, `part_time` → parttime, `contract` → contract.

## Debug/update playbook
1. Verify `ADZUNA_APP_ID` and `ADZUNA_APP_KEY` are set.
2. Confirm the API endpoint responds at `api.adzuna.com/v1/api/jobs/{cc}/search/1`.
3. Check country code mappings for new country support.
4. Validate salary handling for non-predicted salaries.
5. Re-run `go test ./tests/scraper/adzuna` and full suite.
