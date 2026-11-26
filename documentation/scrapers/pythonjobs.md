# pythonjobs

Source id: pythonjobs
Name: Python Jobs
Family: feeds_api
Health: working
Endpoint: GET https://www.python.org/jobs/feed/rss/
Parser: rss
Fetcher: plain
Pagination: none
Profile: none
Credentials: none
Slugs: none
Rate: 20 requests per minute
Concurrency: 1
Robots: allow
Attribution: Python Jobs
Company: Derived from the job link host

## Quirks

- Descriptions are CDATA-wrapped fragments.
- Category and guid fields are not mapped; links identify records.
- A trailing slash in a link is retained in the normalized URL.

## Break and fix log

- 2026-11-26: Tier 1 manifest, synthetic six-file fixture, and offline replay test added.

