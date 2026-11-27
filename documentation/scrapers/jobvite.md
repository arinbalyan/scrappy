# jobvite

Source id: jobvite
Family: ats
Health: requires_slug
Endpoint: GET https://jobs.jobvite.com/api/v2/job-feed/{slug}
Parser: JSON with `requisitions` envelope
Fetcher: plain
Credentials: none
Slugs: example-co (seed placeholder)

## Behavior

- The tenant slug completes the job feed URL; without a seed no request can be built.
- The requisitions envelope holds job items, and empty titles are skipped by the parser.
- Titles come from title, URLs prefer detailUrl then applyUrl, and location prefers city, state, and country with a flat location fallback.
- Company and URL are carried explicitly in the fixture because the historical scraper synthesized the company name and a fallback jobs.jobvite.com URL from the seed and requisition ID.
- Descriptions prefer description over briefDescription and are carried as plain text.

## Quirks

- The historical URL fallback, search-term seed resolution, per-seed deduplication, date parsing, remote detection, IDs, departments, categories, and employment types are outside the normalized record contract and the single-request adapter.
- Boards without a configured seed are skipped; a search phrase is never used as a slug.

## Break and fix log

- 2026-11-26: Ported from historical behavior with a slug manifest and synthetic envelope fixture.
- 2026-11-26: Historical status docs report API timeouts and 20 populated slugs; health stays requires_slug because a tenant seed is required to build the URL.
