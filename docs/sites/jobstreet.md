# jobstreet

## Current integration
- Parser order:
  1. API (SEEK): https://www.seek.com.au/api/jobsearch/v5/search
- Extracted fields: title, company, description, location, salary, classification, date

## Supported knobs
- `results_wanted`
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)
- `search_term`, `location`, `country`
- none

## Constraints and breakpoints
- SEEK-powered API for JobStreet (Malaysia/SE Asia). Uses siteKey=MY-Main. Same search infrastructure as JobsDB with different site key.

## Debug/update playbook
1. Test the API endpoint directly with expected parameters.
2. Verify JSON response structure matches expected fields.
3. Check rate limits and pagination parameters.
4. Re-run `go test ./tests/scraper/jobstreet` and full suite.
