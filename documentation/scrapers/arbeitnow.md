# arbeitnow

Source id: arbeitnow
Family: feeds_api
Health: working
Endpoint: GET https://www.arbeitnow.com/api/job-board-api?page={n}
Parser: json
Fetcher: plain
Pagination: numbered pages, up to 3, stopping when the next link is null
Rate: 20 requests per minute
Concurrency: 1
Robots: allow indexing of public job listings
Attribution: Arbeitnow
Credentials: none
Slugs: none

## Quirks

- Jobs sit under the data envelope selected by parser options.
- Descriptions carry HTML text and are retained as provided.
- The frozen fixture covers one page with a truthy next link.

## Break and fix log

- 2026-11-26: Manifest, fixture, behavior doc, and offline replay recorded during Tier 1 port.
