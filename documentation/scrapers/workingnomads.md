# workingnomads

Source id: workingnomads
Family: feeds_api
Health: working
Endpoint: GET https://www.workingnomads.com/api/exposed_jobs
Parser: json
Fetcher: plain
Pagination: none
Rate: 30 requests per minute
Concurrency: 1
Robots: allow indexing of public job listings
Attribution: Working Nomads
Credentials: none
Slugs: none

## Quirks

- The payload is a top-level array of jobs.
- Company and title use manifest aliases.
- The frozen template uses a base URL substitution for a placeholder-free endpoint.

## Break and fix log

- 2026-11-26: Manifest, fixture, behavior doc, and offline replay recorded during Tier 1 port.
