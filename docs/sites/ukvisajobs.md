# ukvisajobs

## Current integration
- Parser order:
  1. API: https://www.ukvisajobs.com/
- Extracted fields: title, company, description, location, url

## Supported knobs
- `results_wanted`
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)
- `search_term`, `location`, `country`
- no auth required

## Constraints and breakpoints
- UK Visa Jobs API. Lists positions offering visa sponsorship. No auth required.

## Debug/update playbook
1. Test the API endpoint directly with expected parameters.
2. Verify JSON response structure matches expected fields.
3. Check rate limits and pagination parameters.
4. Re-run `go test ./tests/scraper/ukvisajobs` and full suite.
