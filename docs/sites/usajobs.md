# usajobs

## Current integration
- Parser order:
  1. API: https://data.usajobs.gov/api/Search
- Extracted fields: title, organization, location, salary, pay plan, grade, series, description

## Supported knobs
- `results_wanted`
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)
- `search_term`, `location`, `country`
- env var credentials (see Constraints)

## Constraints and breakpoints
- US government job search API. Requires USAJOBS_API_KEY env var. Supports rich government job taxonomy (pay plans, grades, series, agencies).

## Debug/update playbook
1. Test the API endpoint directly with expected parameters.
2. Verify JSON response structure matches expected fields.
3. Check rate limits and pagination parameters.
4. Re-run `go test ./tests/scraper/usajobs` and full suite.
