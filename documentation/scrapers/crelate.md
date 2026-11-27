# crelate

Source id: crelate
Family: keyed
Health: keyed
Endpoint: GET https://app.crelate.com/api3/jobs?published=true&offset=0&limit=25
Parser: JSON root array
Fetcher: plain
Credentials: CRELATE_API_KEY
Slugs: example-co (seed placeholder)

## Behavior

- The jobs endpoint takes an API key in the X-Api-Key header; a tenant seed is still needed to identify the account.
- Titles come from name, with title as the second alias.
- Company and URL come from explicit feed fields; the historical slug-derived company and portal URL are supplied by the fixture payload.
- Location prefers city, then state_province and country, with a flat location as the fallback.
- Descriptions are read from the description field.

## Quirks

- The old scraper built each job URL from the slug and job id and set the company from the seed. URL synthesis and seed-derived fields are outside the adapter, so fixtures carry explicit company and URL values.
- The old scraper used a variable limit and multiple seeds with a delay between them. One request per manifest keeps the frozen limit of 25 and a single seed.
- IDs, remote flags, dates, status, postal codes, and HTML stripping are outside the normalized record contract.

## Break and fix log

- 2026-11-26: Ported from historical behavior with a keyed manifest and synthetic fixture values.
- 2026-11-26: Replay uses a synthetic API key and an explicit portal URL; health stays keyed because the credential is caller-supplied.
