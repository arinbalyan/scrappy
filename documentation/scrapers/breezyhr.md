# breezyhr

Source id: breezyhr
Family: ats
Health: requires_slug
Endpoint: GET https://{slug}.breezy.hr/json
Parser: JSON root array
Fetcher: plain
Credentials: none
Slugs: example-co (seed placeholder)

## Behavior

- The board JSON requires a company slug before a request can be built.
- Title prefers name, then title, matching the historical scraper.
- Location prefers city, state, and country fields in order.
- Fixtures carry explicit URLs because the historical fallback URL is derived from slug and job id.

## Quirks

- Department falls back to category name in the historical scraper, which the adapter does not synthesize. Fixtures put the resolved department directly in department.
- Remote flags and dates from the prior scraper are outside the normalized record contract.

## Break and fix log

- 2026-11-26: Ported from historical behavior with a slug manifest and synthetic fixture.
- 2026-11-26: Live probe returned HTTP 200 for a real public board and HTTP 404 for the placeholder slug.

