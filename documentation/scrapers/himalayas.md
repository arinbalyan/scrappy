# himalayas

Source id: himalayas
Family: feeds_api
Health: working
Endpoint: GET https://himalayas.app/jobs/api?limit={n}&offset={n}
Parser: json
Fetcher: plain
Pagination: offset and limit, page size 20, up to 10 pages
Rate: 20 requests per minute
Concurrency: 1
Robots: allow indexing of public job listings
Attribution: Himalayas
Credentials: none
Slugs: none

## Quirks

- Jobs sit under the jobs envelope alongside count and offset metadata.
- Company, application link, and first location restriction use manifest aliases.
- Salary and seniority fields are present in responses but outside the normalized record.

## Break and fix log

- 2026-11-26: Manifest, fixture, behavior doc, and offline replay recorded during Tier 1 port.
