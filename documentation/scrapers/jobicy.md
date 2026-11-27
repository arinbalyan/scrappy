# jobicy

Source id: jobicy
Family: feeds_api
Health: working
Endpoint: GET https://jobicy.com/api/v2/remote-jobs?count={n}
Parser: json
Fetcher: plain
Pagination: none; count capped at 50
Rate: 30 requests per minute
Concurrency: 1
Robots: allow indexing of public job listings
Attribution: Jobicy
Credentials: none
Slugs: none

## Quirks

- Jobs sit under the jobs envelope.
- Title and company use alternative camel-case aliases.
- Location and description use the job-specific geo and description fields.
- Salary fields in responses are outside the normalized record.

## Break and fix log

- 2026-11-26: Manifest, fixture, behavior doc, and offline replay recorded during Tier 1 port.
