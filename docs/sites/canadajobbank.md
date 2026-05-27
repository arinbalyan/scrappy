# CanadaJobBank

## Current integration
- Parser order:
  1. JSON API at `https://jobbank.api.canada.ca/api/job`
- Extracted fields: _id, Job Title, Original Job Title, Company, City, Province/Territory, Salary Minimum, Salary Maximum, Salary Per, First Posting Date, NOC21 Code Name, Employment Type, Employment Term, Education LOS, Experience Level, Vacancy Count

## Supported knobs
- `results_wanted` — capped at 500
- `search_term` — server-side via `q` parameter
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)

## Constraints and breakpoints
- Free API — no authentication required.
- Single-page query with `limit` parameter (no pagination).
- Description is synthetic: constructed from available fields (occupation, type, term, education, experience, vacancies).
- Company name is **not** available in the API response.
- Salary is always in CAD.
- Job URL follows the pattern `jobbank.gc.ca/jobsearch/jobposting/{id}`.

## Debug/update playbook
1. Verify the API endpoint at `jobbank.api.canada.ca/api/job`.
2. Check the `result` → `records` array structure.
3. Validate salary interval detection (yearly vs hourly).
4. Re-run `go test ./tests/scraper/canadajobbank` and full suite.
