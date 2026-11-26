# ats-ukg

Source id: ats-ukg
Family: ats
Health: requires_slug
Endpoint: GET https://recruiting.ultipro.com/{slug}/OpportunitySearch
Parser: JSON with opportunities envelope
Fetcher: plain
Rate: 30 requests per minute
Concurrency: 1

## Behavior

- The tenant slug completes the URL. The opportunities envelope holds postings.
- URLs prefer applyUrl; fixtures carry explicit URLs where the historical code synthesized a detail URL from ID.
- Company prefers companyName and falls back to the slug. Description prefers description, then shortDescription.
- Location comes from the first entry of locations, falling back to the singular location object.

## Quirks

- Remote inference from formatted addresses and requisition numbers stays outside the adapter.
- Boards without a configured seed cannot build a URL and are skipped.

## Break and fix log

- 2026-11-26: Ported the tenant opportunity search with a slug manifest and synthetic fixture.
- 2026-11-26: Live probe of a historical tenant slug returned 404 HTML; health left at requires_slug because no active public tenant was confirmed.

