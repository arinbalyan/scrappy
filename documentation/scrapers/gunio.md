# gunio

Source id: gunio
Family: html_static
Health: working
Endpoint: GET https://www.gun.io/jobs
Parser: html
Fetcher: plain
Credentials: none
Rate: 20 requests per minute
Concurrency: 2
Robots: allow
Attribution: Gun.io Jobs

## Behavior

- Single GET returns a static HTML page; no browser or emulation profile is used.
- Selectors match each job tile and the heading anchor and company span inside it.
- Public job tiles link to the application site; the fixture uses those absolute URLs.

## Known quirks

- The adapter does not deduplicate URLs within a page.
- Relative URLs stay relative in normalized records.

## Break and fix log
- 2026-11-27: Live validation returned HTTP 200 but no records matched the configured selectors. The static HTML selectors need updating.
- 2026-11-27: Fixed by selecting job tiles rather than individual anchors, and by reading the company span inside each tile; refreshed the synthetic fixture and golden records. Verified via tier 1 replay.

- 2026-11-26: Manifest, generic fixtures, and offline replay recorded during the Tier 1 port.
