# aijobs

Source id: aijobs
Family: html_static
Health: working
Endpoint: GET https://aijobs.ai/remote
Parser: html
Fetcher: plain
Credentials: none
Rate: 20 requests per minute
Concurrency: 2
Robots: allow
Attribution: AI Jobs

## Behavior

- Single GET returns a static HTML page; no browser or emulation profile is used.
- The item selector requires a job-link anchor inside a job-item list entry, so navigation anchors do not match.
- Title and URL come from the anchor; company comes from a sibling span.

## Known quirks

- Selector precision depends on the page keeping the job-item and job-link classes.
- Relative URLs stay relative in normalized records.

## Break and fix log
- 2026-11-27: Live validation returned HTTP 200 but no records matched the configured selectors. The static HTML selectors need updating.

- 2026-11-26: Manifest, generic fixtures, and offline replay recorded during the Tier 1 port.
