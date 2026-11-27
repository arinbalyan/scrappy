# ats-ashby

Source id: ats-ashby
Family: ats
Health: requires_slug
Endpoint: GET https://api.ashbyhq.com/posting-api/job-board/{slug}
Parser: json
Fetcher: plain
Pagination: one request per seed slug; the jobs envelope holds all postings
Rate: 30 requests per minute
Concurrency: 1
Robots: allow public board endpoints; challenged or access-controlled boards are skipped
Attribution: Ashby
Credentials: none
Slugs: example-co (seed placeholder)

## Behavior

- The job-board URL is completed by a seed slug; the slug supplies the company name in normalized records.
- The jobs envelope contains postings. Records use jobUrl, then applyUrl, with the Ashby job path as the former fallback.
- Location text prefers the postal address locality, with the flat location string as the second choice.
- Descriptions prefer plain text, then HTML; HTML is retained as provided in the fixture.
- Employment type, compensation tiers, remote flags, IDs, and dates from the prior scraper are outside the normalized record contract.

## Quirks

- Boards without a configured seed are skipped; a search phrase is never used as a slug.
- Unlisted postings were skipped by the prior scraper; the fixture contains listed postings only.
- A board with no parseable postings is treated as empty rather than an error.

## Break and fix log
- 2026-11-27: Live validation returned HTTP 404 with the example slug. The source remains requires_slug and needs an active board slug for a meaningful probe.

- 2026-11-26: Ported the posting-board JSON response with a slug manifest and synthetic fixture. The source stays requires_slug because a board slug must be supplied before a request can be built.
