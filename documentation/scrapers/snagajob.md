# snagajob

Source id: snagajob
Family: feeds_api
Health: blocked
Endpoint: GET https://www.snagajob.com/api/search?limit=25
Parser: JSON with field aliases
Fetcher: plain
Credentials: none
Rate: 20 requests per minute
Concurrency: 2
Robots: allow
Attribution: Snagajob

## Quirks

- Historical source read jobs from one of jobs, results, or data; this fixture uses jobs. Server-side query parameters and deduplication are outside this adapter.

## Break and fix log
- 2026-11-27: Live validation returned HTTP 403, consistent with the blocked health. The run environment may restrict outbound hosts.

- 2026-11-26: Ported from historical behavior into the manifest adapter with synthetic fixtures.
- 2026-11-26: Live probe returned an HTTP 403 Cloudflare challenge, including with a browser User-Agent, so health changed to blocked.
