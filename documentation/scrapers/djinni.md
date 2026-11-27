# djinni

Source id: djinni
Family: feeds_api
Health: working
Endpoint: GET https://djinni.co/jobs/rss/
Parser: rss
Fetcher: plain
Credentials: none
Rate: 20 requests per minute
Concurrency: 2
Robots: allow
Attribution: Djinni

## Quirks

- Historical source stripped HTML from RSS descriptions; this adapter keeps raw text.
- Historical source filtered by search term and used `guid` for IDs; both are outside this adapter.

## Break and fix log
- 2026-11-27: Live validation returned HTTP 200 but the RSS feed was truncated. The endpoint or feed parser needs review.

- 2026-11-26: Ported from historical behavior into the manifest adapter with synthetic fixtures.


## Live validation

- 2026-11-26: GET returned HTTP 200 with an empty body from the test network; offline replay is synthetic.
