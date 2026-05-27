# jobtechdev

## Current integration
- Parser order:
  1. API: https://jobsearch.api.jobtechdev.se/search
- Extracted fields: title, description, employer, location, occupation, skills, salary, date

## Supported knobs
- `results_wanted`
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)
- `search_term`, `location`, `country`
- no auth required

## Constraints and breakpoints
- Swedish Public Employment Service API (JobTechDev). No auth required. Max limit 100 results per request. Rich taxonomy (occupation, skills, municipalities).

## Debug/update playbook
1. Test the API endpoint directly with expected parameters.
2. Verify JSON response structure matches expected fields.
3. Check rate limits and pagination parameters.
4. Re-run `go test ./tests/scraper/jobtechdev` and full suite.
