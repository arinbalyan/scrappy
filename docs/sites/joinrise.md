# joinrise

## Current integration
- Parser order:
  1. API: https://api.joinrise.ai/api/job/search
- Extracted fields: title, company, description, location, salary, date

## Supported knobs
- `results_wanted`
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)
- `search_term`, `location`, `country`
- no auth required

## Constraints and breakpoints
- JoinRise.ai job search API. No auth required. Paginates with standard offset/limit.

## Debug/update playbook
1. Test the API endpoint directly with expected parameters.
2. Verify JSON response structure matches expected fields.
3. Check rate limits and pagination parameters.
4. Re-run `go test ./tests/scraper/joinrise` and full suite.
