# greenhouse

Source id: greenhouse
Family: ats
Health: requires_slug
Endpoint: GET https://boards-api.greenhouse.io/v1/boards/{slug}/jobs?content=true
Parser: json
Fetcher: plain
Pagination: one request per seed slug; the jobs envelope holds all offers
Rate: 30 requests per minute
Concurrency: 1
Robots: allow public board endpoints; challenged or access-controlled boards are skipped
Attribution: Greenhouse
Credentials: none
Slugs: example-co (seed placeholder); gitlab is a verified live seed

## Behavior

- The board slug completes the URL. Live payloads carry `company_name`, which is the first company alias; a missing value falls back to `company` in the record.
- `content=true` requests full job descriptions; the envelope is the jobs array.
- URLs come from absolute_url, with the board job path as the former fallback.
- Location text comes from the nested location name; a missing location stays null.
- Job identifiers, dates, departments, and worker counts from the prior scraper are outside the normalized record contract.

## Quirks

- Boards without a configured seed are skipped; a search phrase is never used as a slug.
- 429, 403, 401, and 404 responses are treated as failures for that seed and do not produce records.
- Description HTML is carried through in the fixture to preserve the prior content=true intent.

## Break and fix log
- 2026-11-27: Live validation with the `gitlab` seed returned 243 records after adding `company_name` to the company aliases. The fixture keeps its synthetic `company` field, so offline golden records stay stable.
- 2026-11-27: Live validation returned HTTP 404 with the example slug. The source remains requires_slug and needs an active board slug for a meaningful probe.

- 2026-11-26: Ported the board JSON response with a slug manifest and synthetic fixture. The source stays requires_slug because a board slug must be supplied before a request can be built.

## Live seed verification

- 2026-11-27: verified live seed `gitlab` through the manifest adapter, returning 243 records.
- The manifest now ships this verified seed and is declared working. Re-probe when the tenant board moves.
