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
- Selectors match job rows and the anchor and company span within each row.
- The fixture includes a repeated job link to exercise the parser's per-item output.

## Known quirks

- The adapter does not deduplicate URLs; downstream dedup handles repeated links.
- Relative URLs stay relative in normalized records.

## Break and fix log
- 2026-11-27: Live validation returned HTTP 200 but no records matched the configured selectors. The static HTML selectors need updating.

- 2026-11-26: Manifest, generic fixtures, and offline replay recorded during the Tier 1 port.
