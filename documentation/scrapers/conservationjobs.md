# conservationjobs

Source id: conservationjobs
Family: feeds_api
Health: working
Endpoint: GET https://www.conservationjobboard.com/rss
Parser: rss
Fetcher: plain
Credentials: none
Rate: 20 requests per minute
Concurrency: 2
Robots: allow
Attribution: Conservation Jobs

## Quirks

- Feed is niche and may change or challenge automated clients.
- Company is derived from the job URL host when the feed has no company tag.

## Break and fix log

- 2026-11-26: Ported from historical behavior into the manifest adapter with synthetic fixtures.

