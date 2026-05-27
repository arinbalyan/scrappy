# tesla

## Current integration
- Parser order:
  1. API: https://www.tesla.com/careers/boards
- Extracted fields: title, description, location, team, date, category

## Supported knobs
- `results_wanted`
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)
- `search_term`, `location`, `country`
- no auth required

## Constraints and breakpoints
- Two-step: board lookups then board listings then detail pages. Tesla-specific career portal. No auth required. Returns full job description details.

## Debug/update playbook
1. Test the API endpoint directly with expected parameters.
2. Verify JSON response structure matches expected fields.
3. Check rate limits and pagination parameters.
4. Re-run `go test ./tests/scraper/tesla` and full suite.
