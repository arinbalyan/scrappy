# hackernews

Source id: hackernews
Family: feeds_api
Health: working
Endpoint: GET https://hacker-news.firebaseio.com/v0/jobstories.json, then GET https://hacker-news.firebaseio.com/v0/item/{id}.json
Parser: json
Fetcher: plain
Pagination: two-stage fan-out; story ids then items in batches of 10, bounded by twice the wanted count or 45 ids
Rate: 30 requests per minute
Concurrency: 1
Robots: allow indexing of public job listings
Attribution: Hacker News
Credentials: none
Slugs: none

## Quirks

- The story id payload is a top-level array; item payloads are single objects.
- Company is derived from a hiring title pattern upstream. The frozen raw items are official API objects; the replay test adds the derived company before parsing.
- Item aliases map title, company, URL, and text onto normalized records.
- Four story ids produce four frozen item payloads; one has no application URL and is skipped.
- The two-stage flow is represented by frozen raw items and derived items, then replayed by the offline test.

## Break and fix log
- 2026-11-27: Live validation returned HTTP 200 with no records. The official API still exposes `/v0/jobstories.json` and `/v0/item/{id}.json` with `type: "job"` items that carry no company field. Updated the frozen raw items to that shape and moved the hiring-title company derivation into the replay test; golden records are unchanged.

- 2026-11-26: Manifest, fixture, behavior doc, and offline replay recorded during Tier 1 port.
