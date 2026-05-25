# Landing.jobs

## Current integration
- Endpoint: `https://landing.jobs/api/v1/jobs`
- Response shape: bare JSON array of job objects.
- Pagination: offset-based (`offset`, `limit` params), capped at 5 pages (`maxPages=5`) with page size 50.

## Supported knobs
- `search_term` — client-side filter on title, `role_description`, and tags
- `results_wanted`
- global resilience knobs: `retries`, `max_rps`, `site_rps`, proxy config

## Constraints and breakpoints
- API returns a bare array (no envelope wrapping).
- Offset-based pagination with a hard cap of 5 pages (250 results maximum).
- Client-side search term filtering matches title, description, and tags.
- `published_at` is ISO 8601 (RFC 3339 or `2006-01-02T15:04:05`).
- Compensation in `salary_low` / `salary_high` fields (EURO default currency).
- `remote` boolean field maps directly to `IsRemote`.

## Debug/update playbook
1. Confirm endpoint still returns a bare JSON array.
2. Verify `offset` and `limit` pagination still works.
3. Validate `published_at` parsing (try RFC 3339 first, then fallback format).
4. Check optional fields (`salary_low`, `salary_high`, `city`, `country_code`).
5. Re-run `go test ./tests/scraper/landingjobs` and then full suite.
