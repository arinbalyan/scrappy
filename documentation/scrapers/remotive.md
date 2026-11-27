# remotive

Source id: remotive
Family: feeds_api
Health: working
Endpoint: GET https://remotive.com/api/remote-jobs?search={term}
Parser: json
Fetcher: plain
Pagination: none
Rate: 30 requests per minute
Concurrency: 1
Robots: allow indexing of public job listings
Attribution: Remotive
Credentials: none
Slugs: none

## Quirks

- The search term is supplied through a query substitution.
- Jobs sit under the jobs envelope.
- Location uses the candidate-required-location alias.
- The fixture includes one unrelated job to exercise passthrough normalization.

## Break and fix log

- 2026-11-26: Manifest, fixture, behavior doc, and offline replay recorded during Tier 1 port.
