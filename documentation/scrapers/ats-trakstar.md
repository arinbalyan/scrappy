# ats-trakstar

Source id: ats-trakstar
Family: ats
Health: requires_slug
Endpoint: GET https://{slug}.hire.trakstar.com/api/v1/openings
Parser: JSON root array
Fetcher: plain
Credentials: none
Slugs: example-co (seed placeholder)

## Behavior

- The openings endpoint requires a tenant slug before a request can be built.
- Titles come from title and URL from url, with apply_url as the second choice.
- Location prefers city, state, and country, falling back to the flat location field.
- Fixtures carry explicit URLs because the historical slug-derived URL is outside the adapter.

## Quirks

- Compensation, remote flags, dates, departments, and job types are outside the normalized record contract.
- Boards without a configured seed are skipped; a search phrase is never used as a slug.

## Break and fix log
- 2026-11-27: Live validation returned HTTP 401 with the example slug. The source remains requires_slug and needs an active board slug for a meaningful probe.

- 2026-11-26: Ported from historical behavior with a slug manifest and synthetic fixture.
- 2026-11-26: Live probe returned HTTP 301 for the placeholder tenant; health stays requires_slug.


## Live seed verification

- 2026-11-27: verified live seed `chargebee` through the manifest adapter, returning 1 records.
- The route was corrected to the vendor's current public feed and the manifest is declared working.
