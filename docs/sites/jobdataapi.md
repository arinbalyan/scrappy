# jobdataapi

## Current integration
- Parser order:
  1. API: https://jobdataapi.com/api/jobs/
- Extracted fields: title, company, description, location, salary, remote, url

## Supported knobs
- `results_wanted`
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)
- `search_term`, `location`, `country`
- env var credentials (see Constraints)

## Constraints and breakpoints
- JobDataAPI.com REST API. Requires JOB_DATA_API_KEY env var. Aggregated job search across multiple sources.

## Debug/update playbook
1. Test the API endpoint directly with expected parameters.
2. Verify JSON response structure matches expected fields.
3. Check rate limits and pagination parameters.
4. Re-run `go test ./tests/scraper/jobdataapi` and full suite.
