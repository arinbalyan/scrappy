# ats-rippling

Source id: ats-rippling
Family: html_static
Health: requires_slug
Endpoint: GET https://ats.rippling.com/{slug}/jobs
Parser: json (embedded __NEXT_DATA__)
Fetcher: plain
Pagination: none; one board page per seed slug
Rate: 30 requests per minute
Concurrency: 1
Robots: allow public job board pages; challenge pages are skipped
Attribution: Rippling ATS
Credentials: none
Slugs: example-co (seed placeholder)

## Quirks

- The live job array sits under props.pageProps.dehydratedState.queries[0].state.data.items; the first query with items wins in the old scraper. The synthetic fixture represents that first entry as the object key "0" because envelope segments are string keys. A board that puts jobs in a later query needs a new fixture and envelope.
- The parser requires the embedded JSON blob to be a single line, so the fixture keeps the __NEXT_DATA__ payload compact.
- Titles fall back from title to name, and companies fall back to the seed slug.
- Missing per-job URLs are built from the slug and UUID; fixture URLs are absolute.
- HTML in the role and company description fragments is preserved as provided.

## Break and fix log
- 2026-11-27: Live validation returned HTTP 404 with the example slug. The source remains requires_slug and needs an active board slug for a meaningful probe.

- 2026-11-26: Ported the embedded JSON path with a slug manifest and synthetic board fixture. The old scraper's in-page fallback is not represented; the source stays requires_slug until that shape is observed in a fixture.

