# talroo

## Current integration
- Parser order:
  1. API: https://api.jobs2careers.com/api/search.php
- Extracted fields: title, company, location, description, salary, url, date

## Supported knobs
- `results_wanted`
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)
- `search_term`, `location`, `country`
- env var credentials (see Constraints)

## Constraints and breakpoints
- Requires TALROO_PUBLISHER_ID and TALROO_PUBLISHER_PASS env vars. Legacy XML-based API using Jobs2Careers.

## Debug/update playbook
1. Test the API endpoint directly with expected parameters.
2. Verify JSON response structure matches expected fields.
3. Check rate limits and pagination parameters.
4. Re-run `go test ./tests/scraper/talroo` and full suite.
