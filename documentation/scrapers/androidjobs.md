# androidjobs

Source id: androidjobs
Family: feeds_api
Health: working
Endpoint: GET https://androidjobs.io/jobs.rss
Parser: rss
Fetcher: plain
Credentials: none
Rate: 20 requests per minute
Concurrency: 2
Robots: allow
Attribution: Android Jobs

## Behavior

- Single unauthenticated GET returns an RSS channel.
- Item titles use a "Title - Company - City" shape; parser options split on " - " with the title first.
- The third title segment becomes the normalized location.
- Descriptions are kept as text; the prior HTML stripping was a client-side step.

## Known quirks

- A missing separator yields an empty company and no location.
- GUID-based ids from the prior scraper are not part of the normalized record contract.

## Break and fix log

- 2026-11-26: Manifest, generic fixtures, and offline replay recorded during the Tier 1 port.
