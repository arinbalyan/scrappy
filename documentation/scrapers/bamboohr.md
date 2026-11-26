# bamboohr

Source id: bamboohr
Family: ats
Health: requires_slug
Endpoint: GET https://{slug}.bamboohr.com/careers/list
Parser: JSON with result envelope
Fetcher: plain
Credentials: none
Slugs: example-co (seed placeholder)

## Behavior

- The career list endpoint requires a company slug before a request can be built.
- Records use jobOpeningName for title and the result envelope for job arrays.
- The normalized URL falls back to the historical BambooHR careers path when a payload has no URL field, so fixtures carry explicit URLs.
- Location prefers city, state, and country in order.

## Quirks

- The historical scraper derived the URL from the job id and slug. The manifest adapter cannot synthesize fields, so fixtures include explicit URLs.
- Department, compensation, and employment status from the prior scraper are outside the normalized record contract.

## Break and fix log

- 2026-11-26: Ported from historical behavior with a slug manifest and synthetic fixture.
- 2026-11-26: Live probe returned HTTP 200 for a real public tenant and HTTP 302 for the placeholder slug.

