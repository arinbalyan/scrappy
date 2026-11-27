# fountain

Source id: fountain
Family: feeds_api
Health: keyed
Endpoint: GET https://api.fountain.com/v2/openings?limit=25
Parser: JSON with field aliases
Fetcher: plain
Credentials: FOUNTAIN_API_KEY
Rate: 20 requests per minute
Concurrency: 2
Robots: allow
Attribution: Fountain

## Quirks

- Historical source required FOUNTAIN_API_KEY and read openings from the openings array. Salary and dates are outside this adapter.

## Break and fix log
- 2026-11-27: Live validation returned HTTP 401 without FOUNTAIN_API_KEY, confirming the keyed requirement.

- 2026-11-26: Ported from historical behavior into the manifest adapter with synthetic fixtures.
- 2026-11-26: Live probe returned HTTP 401, consistent with the declared FOUNTAIN_API_KEY requirement.
