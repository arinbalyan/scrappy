# railsjobs

Source id: railsjobs
Name: Rails Jobs
Family: feeds_api
Health: working
Endpoint: GET https://jobs.rubyonrails.org/jobs.rss
Parser: rss
Fetcher: plain
Pagination: none
Profile: none
Credentials: none
Slugs: none
Rate: 20 requests per minute
Concurrency: 1
Robots: allow
Attribution: Rails Jobs
Company: Derived from the job link host

## Quirks

- Items carry stable guid values and distinct links.
- Descriptions are CDATA-wrapped text.
- Duplicate records are removed downstream by canonical URL.

## Break and fix log

- 2026-11-26: Tier 1 manifest, synthetic six-file fixture, and offline replay test added.

