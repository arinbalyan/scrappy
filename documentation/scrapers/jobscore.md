# jobscore

Source id: jobscore
Family: ats
Health: requires_slug
Endpoint: GET https://careers.jobscore.com/jobs/{slug}/feed.json?sort=date&limit=25
Parser: JSON root array
Fetcher: plain
Credentials: none
Slugs: example-co (seed placeholder)

## Behavior

- The feed requires a company slug before a request can be built.
- Title comes from title and URL from detail_url, with a derived careers path in the historical scraper.
- Location prefers city, state, and country fields in order.
- Fixtures carry explicit URLs to avoid synthesized fields.

## Quirks

- The historical scraper filtered by search term in the client. The adapter does not apply client-side filters.
- Remote detection from location text, dates, and departments are outside the normalized record contract.

## Break and fix log

- 2026-11-26: Ported from historical behavior with a slug manifest and synthetic fixture.
- 2026-11-26: Live probe returned HTTP 404 for the placeholder slug; health stays requires_slug.

