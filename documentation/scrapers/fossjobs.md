# fossjobs

Source id: fossjobs
Name: Fossjobs
Family: feeds_api
Health: working
Endpoint: GET https://www.fossjobs.net/rss/all/
Parser: rss
Fetcher: plain
Pagination: none
Profile: none
Credentials: none
Slugs: none
Rate: 20 requests per minute
Concurrency: 1
Robots: allow
Attribution: Fossjobs
Company: Derived from the job link host

## Quirks

- Descriptions may be CDATA-wrapped or plain text.
- pubDate values can use timezone abbreviations; these are not normalized yet.
- Item links supply the normalized URL.

## Break and fix log

- 2026-11-26: Tier 1 manifest, synthetic six-file fixture, and offline replay test added.

