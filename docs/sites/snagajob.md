# snagajob

## Current integration
- Parser order:
  1. API: https://www.snagajob.com/api/search
- Extracted fields: title, company, location, pay rate, description, job type

## Supported knobs
- `results_wanted`
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)
- `search_term`, `location`, `country`
- no auth required

## Constraints and breakpoints
- Hourly job search API (Snagajob). No auth required. US-focused hourly/part-time positions.

## Debug/update playbook
1. Test the API endpoint directly with expected parameters.
2. Verify JSON response structure matches expected fields.
3. Check rate limits and pagination parameters.
4. Re-run `go test ./tests/scraper/snagajob` and full suite.
