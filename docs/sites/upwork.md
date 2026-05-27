# upwork

## Current integration
- Parser order:
  1. GraphQL: https://www.upwork.com/api/ (GraphQL)
- Extracted fields: title, description, budget, duration, skills, experience, category

## Supported knobs
- `results_wanted`
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)
- `search_term`, `location`, `country`
- none

## Constraints and breakpoints
- Requires UPWORK_CLIENT_ID and UPWORK_CLIENT_SECRET env vars. OAuth2 authentication with client credentials grant. GraphQL-based search.

## Debug/update playbook
1. Test the API endpoint directly with expected parameters.
2. Verify JSON response structure matches expected fields.
3. Check rate limits and pagination parameters.
4. Re-run `go test ./tests/scraper/upwork` and full suite.
