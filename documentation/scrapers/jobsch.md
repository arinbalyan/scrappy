# jobsch

Source id: jobsch
Family: feeds_api
Health: working
Endpoint: GET https://www.jobs.ch/api/v1/public/search?rows=25
Parser: JSON envelope with field aliases
Fetcher: plain
Credentials: none
Rate: 20 requests per minute
Concurrency: 2
Robots: allow
Attribution: jobs.ch

## Quirks

- Historical source built detail URLs from the first job link with an id fallback; this port reads the English detail link.
- Search term, row count, pagination, publication date, and client-side filtering are outside this adapter.

## Break and fix log

- 2026-11-26: Ported from historical behavior into the manifest adapter with synthetic fixtures.
- 2026-11-26: Live probe returned 200 with a valid documents envelope.
