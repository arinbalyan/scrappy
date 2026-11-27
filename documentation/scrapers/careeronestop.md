# careeronestop

Source id: careeronestop
Family: feeds_api
Health: working
Endpoint: GET https://api.careeronestop.org/v1/jobsearch/anonymous/{search}/{location}/50/relevance/asc/0/25/0
Parser: JSON with Jobs envelope
Fetcher: plain
Rate: 30 requests per minute

## Behavior

- The anonymous job search route returns a Jobs envelope with flat Title, Company, URL, Location, and Description fields.
- Fixtures carry explicit URLs because the historical derived URL is outside the adapter.
- Search term and location substitute through query placeholders.

## Quirks

- The historical comment mentioned an API key, but the implemented route uses the anonymous user segment and sends no key.
- City and state are split from a flat location string in the historical scraper; the normalized record keeps the flat value.

## Break and fix log
- 2026-11-27: Live validation could not reach the public API endpoint. Re-probe from an unrestricted network before changing health.

- 2026-11-26: Ported from historical behavior with a manifest and synthetic fixture.
- 2026-11-26: Live probe timed out after 20 seconds; health left at working because the endpoint may require regional access.

