# loxo

Source id: loxo
Family: ats
Health: blocked
Endpoint: GET https://app.loxo.co/api/{slug}/jobs
Parser: JSON array or jobs envelope
Fetcher: plain
Rate: 30 requests per minute

## Behavior

- The tenant job API accepts a company slug in the path and returns an array or a jobs envelope.
- Fixtures carry explicit URLs because the historical derived URL is outside the adapter.

## Quirks

- The historical scraper used the search term directly as the tenant slug. The manifest uses the slug substitution.
- Structured location objects, compensation, and remote flags are outside the normalized record contract.

## Break and fix log

- 2026-11-26: Ported from historical behavior with a manifest and synthetic fixture.
- 2026-11-26: Live probe of a placeholder tenant returned HTTP 403; health changed to blocked.

