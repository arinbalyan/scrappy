# crunchboard

Source id: crunchboard
Name: Crunchboard
Family: feeds_api
Health: working
Endpoint: GET https://www.crunchboard.com/jobs.rss
Parser: rss
Fetcher: plain
Pagination: none
Profile: none
Credentials: none
Slugs: none
Rate: 20 requests per minute
Concurrency: 1
Robots: allow
Attribution: Crunchboard
Title separator: `" - "`
Company: First title segment; titles use `Company - Title` with `company_first = false`

## Quirks

- Feed company fields are empty; titles use `Company - Title`.
- The title prefix supplies a non-empty company.
- Descriptions are CDATA-wrapped text.

## Break and fix log

- 2026-11-26: Tier 1 manifest, synthetic six-file fixture, and offline replay test added.

