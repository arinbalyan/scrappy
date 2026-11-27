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
- The normalized URL is carried explicitly in the fixture as the public `/careers/{id}/detail` path, since the list response has no URL field.
- Location prefers city, state, and country in order.

## Quirks

- The historical scraper derived the URL from the job id and slug. The manifest adapter cannot synthesize fields, so fixtures include public detail URLs.
- Department, compensation, and employment status from the prior scraper are outside the normalized record contract.

## Break and fix log
- 2026-11-27: Live validation returned HTTP 200 with invalid JSON for the example slug. Public `/careers/list` still returns `result[]` with `jobOpeningName` and `id`, and detail pages use `/careers/{id}/detail`. Updated the fixture and golden to the public URL shape; the placeholder slug explains the invalid response and health stays requires_slug.

- 2026-11-26: Ported from historical behavior with a slug manifest and synthetic fixture.
- 2026-11-26: Live probe returned HTTP 200 for a real public tenant and HTTP 302 for the placeholder slug.

