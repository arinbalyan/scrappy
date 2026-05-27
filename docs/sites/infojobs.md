# infojobs

## Current integration
- Parser order:
  1. API: https://api.infojobs.net/api/7/offer
- Extracted fields: title, company, description, salary, location, contract type, experience, studies

## Supported knobs
- `results_wanted`
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)
- `search_term`, `location`, `country`
- env var credentials (see Constraints)

## Constraints and breakpoints
- Requires INFOJOBS_CLIENT_ID and INFOJOBS_CLIENT_SECRET env vars. Spanish job board (InfoJobs). OAuth-based authentication. Supports rich filtering.

## Debug/update playbook
1. Test the API endpoint directly with expected parameters.
2. Verify JSON response structure matches expected fields.
3. Check rate limits and pagination parameters.
4. Re-run `go test ./tests/scraper/infojobs` and full suite.
