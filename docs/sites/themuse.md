# themuse

## Current integration
- Parser order:
  1. API: https://www.themuse.com/api/public/jobs
- Extracted fields: title, company, description, locations, categories, levels, publication date

## Supported knobs
- `results_wanted`
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)
- `search_term`, `location`, `country`
- no auth required

## Constraints and breakpoints
- The Muse public API. No auth required. Paginated with offset/limit. Rich company metadata (size, industry, logo).

## Debug/update playbook
1. Test the API endpoint directly with expected parameters.
2. Verify JSON response structure matches expected fields.
3. Check rate limits and pagination parameters.
4. Re-run `go test ./tests/scraper/themuse` and full suite.
