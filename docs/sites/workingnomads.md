# workingnomads

## Current integration
- Parser order:
  1. API: https://www.workingnomads.com/api/exposed_jobs
- Extracted fields: title, company, description, location, remote, url

## Supported knobs
- `results_wanted`
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)
- `search_term`, `location`, `country`
- no auth required

## Constraints and breakpoints
- Working Nomads remote job API. No auth required. Curated remote job listings.

## Debug/update playbook
1. Test the API endpoint directly with expected parameters.
2. Verify JSON response structure matches expected fields.
3. Check rate limits and pagination parameters.
4. Re-run `go test ./tests/scraper/workingnomads` and full suite.
