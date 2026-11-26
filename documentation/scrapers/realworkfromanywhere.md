# realworkfromanywhere

Source id: realworkfromanywhere
Name: Real Work From Anywhere
Family: feeds_api
Health: working
Endpoint: GET https://www.realworkfromanywhere.com/rss.xml
Parser: rss
Fetcher: plain
Pagination: none
Profile: none
Credentials: none
Slugs: none
Rate: 20 requests per minute
Concurrency: 1
Robots: allow
Attribution: Real Work From Anywhere
Company: Derived from the job link host

## Quirks

- The board is remote-only; the upstream remote flag is not present in normalized records.
- Embedded job fields in descriptions remain text.
- Descriptions are CDATA-wrapped text.

## Break and fix log

- 2026-11-26: Tier 1 manifest, synthetic six-file fixture, and offline replay test added.

