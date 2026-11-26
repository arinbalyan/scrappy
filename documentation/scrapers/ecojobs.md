# ecojobs

Source id: ecojobs
Family: feeds_api
Health: stale
Endpoint: GET https://www.ecojobs.com/rss.xml
Parser: rss
Fetcher: plain
Credentials: none
Rate: 20 requests per minute
Concurrency: 2
Robots: allow
Attribution: Eco Jobs

## Behavior

- Single unauthenticated GET returns an RSS channel.
- The feed works, but historical status reports zero software-engineer matches; this is a niche board.
- Health is stale because of the narrow niche match rate, not because the feed is unreachable.
- Both CDATA-wrapped and plain item descriptions parse into normalized records.
- The parser derives the company from the link host.

## Known quirks

- No software-specific filter is applied in the adapter; filtering belongs to callers.
- Slug-based ids from the prior scraper are not part of the normalized record contract.

## Break and fix log

- 2026-11-26: Manifest, generic fixtures, and offline replay recorded during the Tier 1 port.
