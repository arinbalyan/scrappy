# startupjobs

Source id: startupjobs
Family: feeds_api
Health: blocked
Endpoint: GET https://startup.jobs/api/jobs
Parser: JSON with field aliases
Fetcher: plain
Credentials: none
Rate: 20 requests per minute
Concurrency: 2
Robots: allow
Attribution: Startup Jobs

## Quirks

- Historical source accepted a bare array or a jobs/items wrapper; this port covers the bare array. Client-side filtering and date parsing are outside this adapter.

## Break and fix log
- 2026-11-27: The latest live sweep kept startupjobs in the blocked section
  with HTTP 403. The challenge seen on 2026-11-26 is unresolved; health stays
  blocked until a browser-capable probe from an unrestricted network returns
  records.
- 2026-11-27: Live validation returned HTTP 403, consistent with the blocked health. The run environment may restrict outbound hosts.

- 2026-11-26: Ported from historical behavior into the manifest adapter with synthetic fixtures.
- 2026-11-26: Live probe returned a Cloudflare challenge; health changed to blocked.
