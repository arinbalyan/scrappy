# manatal

Source id: manatal
Family: ats
Health: requires_slug
Endpoint: GET https://api.manatal.com/open/v1/career-page/{slug}/jobs/
Parser: JSON with results envelope
Fetcher: plain
Credentials: none
Slugs: example-co (seed placeholder)

## Behavior

- The open career page API requires a company slug before a request can be built.
- Title comes from position_name and URL prefers apply_url, then career_page_url.
- Location prefers city, state, and country fields in order.
- Fixtures carry explicit URLs because the historical endpoint-derived URL is outside the adapter.

## Quirks

- The historical scraper read a next-page cursor. The manifest adapter has no cursor executor, so pagination remains none.
- Compensation, dates, departments, and job types are outside the normalized record contract.

## Break and fix log

- 2026-11-26: Ported from historical behavior with a slug manifest and synthetic fixture.
- 2026-11-26: Live probe returned HTTP 404 for the placeholder slug; health stays requires_slug.

