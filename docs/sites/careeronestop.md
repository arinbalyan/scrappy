# CareerOneStop

## Current integration
- Parser order:
  1. JSON API at `https://api.careeronestop.org/v1/jobsearch`
- Extracted fields: JvId, Title, Company, URL, Location, Description, DatePosted

## Supported knobs
- `results_wanted`
- `search_term` — server-side via URL path
- `location` — server-side via URL path
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)

## Constraints and breakpoints
- **NOT publicly accessible** — this API requires an API key. The code uses the `anonymous` user ID path. If the API becomes gated, this will break.
- API path format: `/v1/jobsearch/{userId}/{keyword}/{location}/{radius}/{sortColumns}/{sortOrder}/{startRecord}/{pageSize}/{days}`.
- Single-page query — no pagination support.
- Location parsed as "City, State" from comma-separated string.
- Remote detection is heuristic: looks for "remote" in location string.

## Debug/update playbook
1. Verify the API endpoint at `api.careeronestop.org/v1/jobsearch` is still accessible.
2. Check the `Jobs` array structure in the response.
3. Update `anonymous` userId if required.
4. Re-run `go test ./tests/scraper/careeronestop` and full suite.
