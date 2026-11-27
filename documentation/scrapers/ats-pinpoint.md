# ats-pinpoint

Source id: ats-pinpoint
Family: ats
Health: requires_slug
Endpoint: GET https://{slug}.pinpointhq.com/postings.json
Parser: JSON with data envelope
Fetcher: plain
Rate: 30 requests per minute
Concurrency: 1

## Behavior

- The tenant slug completes the subdomain and supplies the company when attributes.company_name is absent.
- The data envelope holds listings. Fixtures use explicit upstream URLs because URL composition from an ID is outside the adapter.
- Location and description come from attributes.location and attributes.description.
- The historical scraper also accepted a bare array; the manifest commits to the documented data envelope.

## Quirks

- Boards without a configured seed cannot build a URL and are skipped.
- Remote inference and publication dates stay outside the normalized record contract.

## Break and fix log
- 2026-11-27: Live validation returned HTTP 404 with the example slug. The source remains requires_slug and needs an active board slug for a meaningful probe.

- 2026-11-26: Ported the tenant postings feed with a slug manifest and synthetic fixture.
- 2026-11-26: Live probe of a historical tenant slug returned 404 HTML; health left at requires_slug because no active public tenant was confirmed.

