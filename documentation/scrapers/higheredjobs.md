# higheredjobs

Source id: higheredjobs
Family: feeds_api
Health: blocked
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
- The plain fetcher received an Incapsula challenge page during live validation; browser emulation is needed before this source can serve records.
- The public category feed uses `catID` to select a job category; 68 covers information technology.

## Break and fix log
- 2026-11-27: Live validation returned HTTP 200 but the RSS feed was malformed HTML. The endpoint or feed parser needs review.
- 2026-11-27: Fixed the feed URL to the current category feed and refreshed the synthetic RSS fixture and golden records. Health remains blocked because the plain fetcher still receives a challenge page; verified via tier 5 replay.

- 2026-11-26: Ported from historical behavior into the manifest adapter with synthetic fixtures.
- 2026-11-26: Live probe returned an Incapsula challenge; health changed to blocked.
