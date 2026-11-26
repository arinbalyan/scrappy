# berlinstartupjobs

Source id: berlinstartupjobs
Family: feeds_api
Health: working
Endpoint: GET https://berlinstartupjobs.com/feed/
Parser: rss with title pattern ` // `
Fetcher: plain
Credentials: none
Rate: 20 requests per minute
Concurrency: 2
Robots: allow
Attribution: Berlin Startup Jobs

## Quirks

- Feed titles encode the employer after a ` // ` separator.
- No client-side filtering or detail enrichment is available in this adapter.

## Break and fix log

- 2026-11-26: Ported from historical behavior into the manifest adapter with synthetic fixtures.

