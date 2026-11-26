# devopsjobs

Source id: devopsjobs
Name: Devopsjobs
Family: feeds_api
Health: working
Endpoint: GET https://devopsjobs.io/jobs.rss
Parser: rss
Fetcher: plain
Pagination: none
Profile: none
Credentials: none
Slugs: none
Rate: 20 requests per minute
Concurrency: 1
Robots: allow
Attribution: Devopsjobs
Title separator: `" - "`
Company: Second title segment; titles use `Title - Company - City` with `company_first = false`

## Quirks

- Titles use `Title - Company - City`.
- The third segment becomes the location.
- Descriptions are CDATA-wrapped text.

## Break and fix log

- 2026-11-26: Tier 1 manifest, synthetic six-file fixture, and offline replay test added.

