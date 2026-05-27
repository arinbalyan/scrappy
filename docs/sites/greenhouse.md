# greenhouse

## Current integration
- Parser order:
  1. API: https://boards-api.greenhouse.io/v1/boards/{slug}/jobs
- Extracted fields: title, company, description, location, departments, metadata (office, remote)

## Supported knobs
- `results_wanted`
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)
- `search_term`, `location`, `country`
- no auth required

## Constraints and breakpoints
- Greenhouse board API. Slug-based URL via ats.ResolveSeeds. Source: SCRAPPY_GREENHOUSE_SEEDS or config/company_slugs.yaml. No auth required for public board API.

## Debug/update playbook
1. Test the API endpoint directly with expected parameters.
2. Verify JSON response structure matches expected fields.
3. Check rate limits and pagination parameters.
4. Re-run `go test ./tests/scraper/greenhouse` and full suite.
