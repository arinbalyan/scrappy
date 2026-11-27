# dice

Source id: dice
Family: feeds_api
Health: keyed
Endpoint: GET https://job-search-api.svc.dhigroupinc.com/v1/dice/jobs/search?limit=25
Parser: JSON with field aliases
Fetcher: plain
Credentials: SCRAPPY_DICE_API_KEY
Rate: 20 requests per minute
Concurrency: 2
Robots: allow
Attribution: Dice

## Quirks

- Historical source sent a client-derived x-api-key header and read jobs from data with an HTML fallback. The HTML fallback is outside this adapter.

## Break and fix log
- 2026-11-27: Live validation could not build the request without SCRAPPY_DICE_API_KEY, confirming the keyed requirement.

- 2026-11-26: Ported from historical behavior into the manifest adapter with synthetic fixtures.
- 2026-11-26: Live probe returned HTTP 403 without a key, confirming that the x-api-key header is required.
