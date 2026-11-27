# ats-teamtailor

Source id: ats-teamtailor
Family: ats
Health: requires_slug
Endpoint: GET https://career.teamtailor.com/widget/jobs/{slug}
Parser: JSON with data envelope
Fetcher: plain
Rate: 30 requests per minute
Concurrency: 1

## Behavior

- The widget returns JSON:API items in a data envelope.
- URLs prefer links.careersite-url, then attributes.apply-url, then attributes.external-url.
- Fixtures carry explicit careersite URLs and company objects because the historical code synthesized them from the slug.
- Location comes from attributes.city and description from attributes.body.

## Quirks

- Department relationships and remote flags stay outside the normalized record contract.
- Boards without a configured seed cannot build a URL and are skipped.

## Break and fix log
- 2026-11-27: Live validation returned HTTP 404 with the example slug. The source remains requires_slug and needs an active board slug for a meaningful probe.

- 2026-11-26: Ported the tenant widget endpoint with a slug manifest and synthetic fixture.
- 2026-11-26: Live probe of a historical tenant slug returned 404 HTML; health left at requires_slug because no active public tenant was confirmed.


## Live seed verification

- 2026-11-27: verified live seed `polestar` through the manifest adapter, returning 15 records.
- The route was corrected to the vendor's current public feed and the manifest is declared working.
