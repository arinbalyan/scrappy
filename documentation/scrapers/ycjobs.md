# ycjobs

Source id: ycjobs
Family: keyed
Health: blocked
Endpoint: GET https://www.ycombinator.com/jobs
Parser: html
Fetcher: plain
Credentials: none
Rate: 20 requests per minute
Concurrency: 2
Robots: allow
Attribution: YC Jobs

## Behavior and limitation

- The prior flow discovered a frontend search key from a public page and POSTed to an index query API, with an HTML fallback when discovery failed. The Request type in this crate exposes only GET, and the plain fetcher has no POST path, so the derived-key query cannot be ported.
- The manifest and fixtures replay only the observable GET/HTML fallback page, parsed with selectors for job links and company spans.
- No key value, derived or otherwise, appears in the manifest, fixtures, or docs.
- Health is blocked because the working path needs POST support and derived-key handling that this adapter does not provide.

## Known quirks

- No key discovery or in-memory key handling is implemented.
- Relative URLs stay relative in normalized records.

## Break and fix log

- 2026-11-26: GET/HTML fallback representation, generic fixtures, and offline replay recorded during the Tier 1 port; POST and derived-key flow remain unported.
