# FindWork

## Current integration
- Parser order:
  1. JSON API at `https://findwork.dev/api/jobs/`
- Extracted fields: id, role, company_name, company_num_employees, employment_type, location, remote, logo, url, text, date_posted, keywords, source

## Supported knobs
- `results_wanted`
- `search_term` — server-side via `search` param
- `location` — server-side via `location` param
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)

## Constraints and breakpoints
- **Requires `FINDWORK_API_KEY`** environment variable — register at `findwork.dev/developers/`.
- API uses Token authentication (`Authorization: Token {key}`).
- Pagination is cursor/URL-based via `next` field.
- Rate-limited via jitter sleep between 200–500ms (~3 req/s).
- Keywords array is mapped to `Skills`.
- Company name is available from `company_name` field.
- Description (`text`) is plain text — simple tag stripping applied.

## Debug/update playbook
1. Verify `FINDWORK_API_KEY` is set.
2. Check the API at `findwork.dev/api/jobs/` responds.
3. Validate pagination via `next` URL resolution.
4. Re-run `go test ./tests/scraper/findwork` and full suite.
