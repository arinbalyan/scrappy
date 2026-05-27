# hiringcafe

## Current integration
- Parser order:
  1. API: https://hiring.cafe/
- Extracted fields: title, company, description, location, salary, remote, date

## Supported knobs
- `results_wanted`
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)
- `search_term`, `location`, `country`
- no auth required

## Constraints and breakpoints
- JSON REST API at hiring.cafe. No auth required. Supports search term and location filtering. Paginates via cursor/offset.

## Debug/update playbook
1. Test the API endpoint directly with expected parameters.
2. Verify JSON response structure matches expected fields.
3. Check rate limits and pagination parameters.
4. Re-run `go test ./tests/scraper/hiringcafe` and full suite.
