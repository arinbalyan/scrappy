# mycareersfuture

## Current integration
- Parser order:
  1. API: https://api.mycareersfuture.gov.sg/v2/jobs
- Extracted fields: title, company, description, location, salary

## Supported knobs
- `results_wanted`
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)
- `search_term`, `location`, `country`
- no auth required

## Constraints and breakpoints
- Singapore government jobs API (MyCareersFuture). No auth for public search. Government employment platform for Singapore.

## Debug/update playbook
1. Test the API endpoint directly with expected parameters.
2. Verify JSON response structure matches expected fields.
3. Check rate limits and pagination parameters.
4. Re-run `go test ./tests/scraper/mycareersfuture` and full suite.
