# nofluffjobs

## Current integration
- Parser order:
  1. API: https://nofluffjobs.com/api/posting
- Extracted fields: title, company, description, location, salary, seniority

## Supported knobs
- `results_wanted`
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)
- `search_term`, `location`, `country`
- no auth required

## Constraints and breakpoints
- Polish tech job board API. No auth required. JSON REST API with search and filtering.

## Debug/update playbook
1. Test the API endpoint directly with expected parameters.
2. Verify JSON response structure matches expected fields.
3. Check rate limits and pagination parameters.
4. Re-run `go test ./tests/scraper/nofluffjobs` and full suite.
