# talroo

Source id: talroo
Family: feeds_api
Health: keyed
Endpoint: GET https://api.jobs2careers.com/api/search.php?limit=25
Parser: JSON with field aliases
Fetcher: plain
Credentials: TALROO_PUBLISHER_ID, TALROO_PUBLISHER_PASS
Rate: 20 requests per minute
Concurrency: 2
Robots: allow
Attribution: Talroo

## Quirks

- Historical source sent publisher credentials as query parameters and read jobs from the jobs array. This port declares credential names only.

## Break and fix log
- 2026-11-27: Live validation returned HTTP 400 without TALROO_PUBLISHER_ID and TALROO_PUBLISHER_PASS, confirming the keyed requirement.

- 2026-11-26: Ported from historical behavior into the manifest adapter with synthetic fixtures.
- 2026-11-26: Live probe returned HTTP 400 because publisher ID and password were not supplied.
