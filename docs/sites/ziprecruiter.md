# ziprecruiter

## Current integration
- Parser order:
  1. API: https://api.ziprecruiter.com/jobs-app/jobs
- Extracted fields: title, company, location, salary, description, remote, date

## Supported knobs
- `results_wanted`
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)
- `search_term`, `location`, `country`
- no auth required

## Constraints and breakpoints
- ZipRecruiter public API. No auth required. Supports location filtering and search terms. Returns rich job data including compensation breakdown.

## Debug/update playbook
1. Test the API endpoint directly with expected parameters.
2. Verify JSON response structure matches expected fields.
3. Check rate limits and pagination parameters.
4. Re-run `go test ./tests/scraper/ziprecruiter` and full suite.
