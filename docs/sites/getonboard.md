# GetOnBoard

## Current integration
- Parser order:
  1. JSON API at `https://www.getonbrd.com/api/v0/search/jobs`
- Extracted fields: id, type, attributes (title, description, company, logo, min_salary, max_salary, remote, seniority, published_at, countries, location_cities, tags), links (public_url)

## Supported knobs
- `results_wanted` — capped at 100
- `search_term` — server-side via `query` param
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)

## Constraints and breakpoints
- Free public API — no authentication required.
- Single-page query with `per_page` param (max 50 per request, max 100 results total).
- JSON-API format: `data` array with `id`, `type`, `attributes`, `links`.
- `location_cities` may be either `[]string` or a JSON-API relationship object — handled via dynamic unmarshal.
- `seniority` may be either a plain string or an object `{"name":"..."}` — handled via dynamic unmarshal.
- Published date is in Unix timestamp (seconds).
- Salary is always in USD, yearly interval.

## Debug/update playbook
1. Verify the API at `getonbrd.com/api/v0/search/jobs`.
2. Check `data` → `attributes` structure for any field changes.
3. Validate location_cities and seniority field format variations.
4. Re-run `go test ./tests/scraper/getonboard` and full suite.
