# higheredjobs

Source id: higheredjobs
Family: feeds_api
Health: working
Endpoint: GET https://www.higheredjobs.com/rss/categoryFeed.cfm?catID=68
Parser: RSS with title pattern splitting
Fetcher: plain
Credentials: none
Rate: 20 requests per minute
Concurrency: 2
Robots: allow
Attribution: HigherEdJobs

## Quirks

- Historical source parsed RSS items with custom namespaced fields; this port splits the title into title and company.
- Client-side search filtering, date parsing, salary, and category are outside this adapter.
- The 2026-11-26 probe received an Incapsula challenge. Later probes returned 17 records through the plain fetcher.
- The public category feed uses `catID` to select a job category; 68 covers information technology.

## Break and fix log
- 2026-11-27: Live validation returned 17 records from the category feed. Updated health and replay contract to working.
- 2026-11-27: Fixed the feed URL to the current category feed and refreshed the synthetic RSS fixture and golden records; verified via tier 5 replay.

- 2026-11-26: Ported from historical behavior into the manifest adapter with synthetic fixtures.
- 2026-11-26: Live probe returned an Incapsula challenge; health changed to blocked.
