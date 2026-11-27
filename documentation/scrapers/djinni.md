# djinni

Source id: djinni
Family: feeds_api
Health: stale
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
- The RSS endpoint remains `https://djinni.co/jobs/rss/`, but live probes return an empty body.

## Break and fix log
- 2026-11-27: The latest live sweep kept djinni in the stale section. The
  zero-byte RSS capture is the reason; no replacement feed was verified.
- 2026-11-27: Both live captures were zero bytes and validation reported `invalid RSS feed: unexpected Event::Eof`. Marked the source stale; the full feed is unavailable from this network.
- 2026-11-27: Live validation returned HTTP 200 but the RSS feed was truncated. The endpoint or feed parser needs review.

- 2026-11-26: Ported from historical behavior into the manifest adapter with synthetic fixtures.


## Live validation

- 2026-11-27: GET returned HTTP 200 with a zero-byte body on both probes; offline replay is synthetic.
- 2026-11-26: GET returned HTTP 200 with an empty body from the test network; offline replay is synthetic.
