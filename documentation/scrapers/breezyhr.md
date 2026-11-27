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
- 2026-11-27: Live validation returned HTTP 404 with the example slug. The source remains requires_slug and needs an active board slug for a meaningful probe.

- 2026-11-26: Ported from historical behavior with a slug manifest and synthetic fixture.
- 2026-11-26: Live probe returned HTTP 200 for a real public board and HTTP 404 for the placeholder slug.


## Live seed verification

- 2026-11-27: verified live seed `thirdchannel-inc` through the manifest adapter, returning 757 records.
- The manifest now ships this verified seed and is declared working. Re-probe when the tenant board moves.
