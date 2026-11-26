# comeet

Source id: comeet
Family: keyed
Health: keyed
Endpoint: GET https://www.comeet.co/careers-api/2.0/company/{slug}/positions?token={key:COMEET_API_TOKEN}&details=true
Parser: JSON root array
Fetcher: plain
Credentials: COMEET_API_TOKEN
Slugs: example-co (seed placeholder)

## Behavior

- The public positions endpoint requires a company slug and a public API token before a request can be built.
- Title comes from name and company from company_name, with the slug as a downstream fallback.
- URL prefers url_active_page, then url.
- Location comes from location.name and fixtures use remote text where applicable.

## Quirks

- The historical scraper targeted www.comeet.com with an empty token. The documented API uses www.comeet.co with a company UID and a token query parameter.
- Descriptions were joined from a details array by the prior scraper. The manifest reads the first detail value when a direct description is absent.

## Break and fix log

- 2026-11-26: Ported from historical behavior with a slug manifest and synthetic fixture.
- 2026-11-26: Live probe returned HTTP 200 for the documented API host with a public token and a real company UID.

