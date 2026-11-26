# jobspresso

Source id: jobspresso
Name: Jobspresso
Family: feeds_api
Health: working
Endpoint: GET https://jobspresso.co/feed/?post_type=job_listing
Parser: rss
Fetcher: plain
Pagination: none
Profile: none
Credentials: none
Slugs: none
Rate: 20 requests per minute
Concurrency: 1
Robots: allow
Attribution: Jobspresso
Title separator: `": "`
Company: Title prefix before the colon with `company_first = true`

## Quirks

- Titles use `Company: Title`.
- Links are job-listing pages with numeric ids.
- Descriptions are CDATA-wrapped text.

## Break and fix log

- 2026-11-26: Tier 1 manifest, synthetic six-file fixture, and offline replay test added.

