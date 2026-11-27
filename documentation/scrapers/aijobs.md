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
- Each listing is a job card anchor with class jobcardStyle1, so navigation anchors do not match.
- Title and company come from descendants of that card; the card anchor itself supplies the URL.

## Known quirks

- Selector precision depends on the page keeping the job-card and title classes.
- Relative URLs stay relative in normalized records.

## Break and fix log
- 2026-11-27: Live validation returned HTTP 200 but no records matched the old nested container selectors.
- 2026-11-27: Fixed by selecting the job card anchor directly and allowing a selector to match the item node itself; refreshed the synthetic fixture and golden records. Verified via tier 1 replay and a live run that returned 22 records.

- 2026-11-26: Manifest, generic fixtures, and offline replay recorded during the Tier 1 port.
