# icims

Source id: icims
Family: ats
Health: requires_slug
Endpoint: GET https://{slug}.icims.com/jobs/search?pr=0&o=0&mode=job&iis=Internet
Parser: JSON with `jobs` envelope
Fetcher: plain
Credentials: none
Slugs: example-co (seed placeholder)

## Behavior

- The tenant slug completes the gateway search URL; without a seed no request can be built.
- The gateway envelope holds job items, and empty titles are skipped by the parser.
- Titles come from title, company and URL are carried explicitly in the fixture, and location prefers location over city and state.
- The historical scraper derived company from the seed and expanded relative URLs or built a job URL from the ID. Those synthesized values are outside the adapter, so fixtures carry explicit company and URL fields.
- Descriptions come directly from the description field when present.

## Quirks

- The historical offset pagination, search-term seed resolution, per-seed deduplication, date parsing, remote detection, categories, and IDs are outside the normalized record contract and the single-request adapter.
- Boards without a configured seed are skipped; a search phrase is never used as a slug.

## Break and fix log

- 2026-11-26: Ported from historical behavior with a slug manifest and synthetic envelope fixture.
- 2026-11-26: Historical status docs report API slowness and 39 populated slugs; health stays requires_slug because a tenant seed is required to build the URL.
