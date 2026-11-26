# hasjob

Source id: hasjob
Name: Hasjob
Family: feeds_api
Health: working
Endpoint: GET https://hasjob.co/feed
Parser: rss
Fetcher: plain
Pagination: none
Profile: none
Credentials: none
Slugs: none
Rate: 20 requests per minute
Concurrency: 1
Robots: allow
Attribution: Hasjob
Company: Derived from the alternate link host

## Quirks

- The endpoint returns an Atom document although the manifest parser is `rss`.
- The alternate link supplies the URL and the `<location>` element supplies location.
- The published timestamp is not mapped into the normalized record.

## Break and fix log

- 2026-11-26: Tier 1 manifest, synthetic six-file fixture, and offline replay test added.

