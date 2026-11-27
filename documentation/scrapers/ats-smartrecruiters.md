# ats-smartrecruiters

Source id: ats-smartrecruiters
Family: ats
Health: requires_slug
Endpoint: GET https://api.smartrecruiters.com/v1/companies/{slug}/jobs?offset=0&limit={query:limit}
Parser: json
Fetcher: plain
Pagination: offset pages of 100 per seed slug, stopping on a short page
Rate: 30 requests per minute
Concurrency: 1
Robots: allow public board endpoints; challenged or access-controlled boards are skipped
Attribution: SmartRecruiters
Credentials: none
Slugs: example-co (seed placeholder)

## Behavior

- The company slug completes the URL and supplies the fallback company name in normalized records.
- The content envelope holds postings; an explicit company name takes precedence over the slug.
- URLs come from ref, with the SmartRecruiters job path as the former fallback.
- Location text is read from city, then region, then country in the nested location object.
- Descriptions come from the job description section only; qualifications and additional information from the prior scraper are outside the record contract.
- IDs, departments, remote flags, dates, and employment types are also outside the normalized record contract.

## Quirks

- Boards without a configured seed are skipped; a search phrase is never used as a slug.
- The limit placeholder is a page size, not the number of records to return.
- Description text may contain HTML; the fixture keeps it as provided.

## Break and fix log
- 2026-11-27: Live validation returned HTTP 404 with the example slug. The source remains requires_slug and needs an active board slug for a meaningful probe.

- 2026-11-26: Ported the company postings JSON response with a slug manifest and synthetic fixture. The source stays requires_slug because a company slug must be supplied before a request can be built.
