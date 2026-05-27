# EchoJobs

## Current integration
- Parser order:
  1. JSON API at `https://echojobs.io/api/jobs`
- Extracted fields: id, title, company, company_logo, url, description, job_type, location, is_remote, tags, date_posted, salary_min, salary_max, salary_currency

## Supported knobs
- `results_wanted`
- `search_term` — client-side filter on title + tags
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)

## Constraints and breakpoints
- Single API fetch — no pagination.
- API supports two response formats: bare JSON array, or `{"jobs":[...]}` / `{"data":[...]}` wrapper object.
- Search term is client-side (case-insensitive substring match on title + tags).
- Dynamic JSON decoding via `map[string]any` — robust to schema changes but loses compile-time type safety.
- Compensation extracted from `salary_min`, `salary_max`, `salary_currency` fields.
- Skills extracted from `tags` array.

## Debug/update playbook
1. Verify the API endpoint at `echojobs.io/api/jobs`.
2. Check if the response is a JSON array or wrapped object.
3. Validate field name mappings for any new/changed fields.
4. Re-run `go test ./tests/scraper/echojobs` and full suite.
