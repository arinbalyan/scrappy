# startupjobs

## Current integration
- Parser order:
  1. API + RSS: https://startup.jobs/api/jobs
- Extracted fields: title, company, description, location, remote, salary

## Supported knobs
- `results_wanted`
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)
- `search_term`, `location`, `country`
- none

## Constraints and breakpoints
- Dual approach: primary JSON API at startup.jobs, fallback to JSON feed. Startup-focused job board.

## Debug/update playbook
1. Test the API endpoint directly with expected parameters.
2. Verify JSON response structure matches expected fields.
3. Check rate limits and pagination parameters.
4. Re-run `go test ./tests/scraper/startupjobs` and full suite.
