# coroflot

Source id: coroflot
Family: feeds_api
Health: working
Endpoint: GET http://feeds.feedburner.com/Coroflot/AllJobs
Parser: RSS with title pattern splitting
Fetcher: plain
Credentials: none
Rate: 20 requests per minute
Concurrency: 2
Robots: allow
Attribution: Coroflot

## Quirks

- Historical source parsed RSS items and extracted location from the description HTML; this port uses plain RSS records. Location extraction, date parsing, and HTML stripping are outside this adapter.

## Break and fix log

- 2026-11-26: Ported from historical behavior into the manifest adapter with synthetic fixtures.
- 2026-11-26: Live probe returned 200 with a valid RSS feed.
