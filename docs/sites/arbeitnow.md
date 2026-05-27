# Arbeitnow

## Current integration
- Parser order:
  1. JSON API at `https://www.arbeitnow.com/api/job-board-api`
- Extracted fields: slug, company_name, title, description, remote, url, tags, location, created_at

## Supported knobs
- `results_wanted`
- `search_term` — client-side filter on title + description + tags
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)

## Constraints and breakpoints
- Pagination is page-based with a hard cap of 3 pages.
- API returns `links.next` to indicate more pages — if `next` is null, pagination stops.
- Search term filtering is client-side (case-insensitive substring match).
- ID is derived from the `slug` field, falling back to URL hash.
- Created_at is a Unix timestamp in seconds.
- Description is HTML — stripped to plain text.

## Debug/update playbook
1. Verify the API endpoint at `arbeitnow.com/api/job-board-api`.
2. Check `apiResp` structure for any field changes.
3. Validate pagination via `links.next` behavior.
4. Re-run `go test ./tests/scraper/arbeitnow` and full suite.
