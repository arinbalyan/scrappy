# jobindex

## Current integration
- Parser order:
  1. API: https://www.jobindex.dk/jobsoegning
- Extracted fields: title, company, location, description, date

## Supported knobs
- `results_wanted`
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)
- `search_term`, `location`, `country`
- no auth required

## Constraints and breakpoints
- Danish job board API (Jobindex.dk). No auth required. Supports search term and location filtering.

## Debug/update playbook
1. Test the API endpoint directly with expected parameters.
2. Verify JSON response structure matches expected fields.
3. Check rate limits and pagination parameters.
4. Re-run `go test ./tests/scraper/jobindex` and full suite.
