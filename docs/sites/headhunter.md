# headhunter

## Current integration
- Parser order:
  1. API: https://api.hh.ru/vacancies
- Extracted fields: title, employer, description, salary, location, employment type, experience

## Supported knobs
- `results_wanted`
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)
- `search_term`, `location`, `country`
- no auth required

## Constraints and breakpoints
- HeadHunter (hh.ru) API. Russian job board. No auth required for basic search. Rate limits apply. Supports location and search term filtering.

## Debug/update playbook
1. Test the API endpoint directly with expected parameters.
2. Verify JSON response structure matches expected fields.
3. Check rate limits and pagination parameters.
4. Re-run `go test ./tests/scraper/headhunter` and full suite.
