# nofluffjobs

Source id: nofluffjobs
Family: feeds_api
Health: working
Endpoint: GET https://nofluffjobs.com/api/rss
Parser: json with field aliases
Fetcher: plain
Credentials: none
Rate: 20 requests per minute
Concurrency: 2
Robots: allow
Attribution: NoFluffJobs

## Quirks

- Historical source accepted several response shapes; this port covers the bare-array shape.
- Salary and category fields are outside this adapter.

## Break and fix log

- 2026-11-26: Ported from historical behavior into the manifest adapter with synthetic fixtures.

