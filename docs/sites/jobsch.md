# jobsch

## Current integration
- Parser order:
  1. API: https://www.jobs.ch/api/v1/public/search
- Extracted fields: title, company, description, location, salary, date

## Supported knobs
- `results_wanted`
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)
- `search_term`, `location`, `country`
- no auth required

## Constraints and breakpoints
- Swiss job board API. No auth required. Public REST API with search and pagination.

## Debug/update playbook
1. Test the API endpoint directly with expected parameters.
2. Verify JSON response structure matches expected fields.
3. Check rate limits and pagination parameters.
4. Re-run `go test ./tests/scraper/jobsch` and full suite.
