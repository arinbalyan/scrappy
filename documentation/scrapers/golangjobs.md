# golangjobs

Source id: golangjobs
Name: Golang Jobs
Family: feeds_api
Health: working
Endpoint: GET https://www.golangprojects.com/rss.xml
Parser: rss
Fetcher: plain
Pagination: none
Profile: none
Credentials: none
Slugs: none
Rate: 20 requests per minute
Concurrency: 1
Robots: allow
Attribution: Golang Jobs
Company: Derived from the job link host

## Quirks

- Category names are present in the feed but not used as a filter in the adapter.
- Links end in `.html` and are normalized verbatim.
- Descriptions are CDATA-wrapped text.

## Break and fix log

- 2026-11-26: Tier 1 manifest, synthetic six-file fixture, and offline replay test added.

