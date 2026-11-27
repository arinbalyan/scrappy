# ats-workable

Source id: ats-workable
Family: ats
Health: requires_slug
Endpoint: GET https://apply.workable.com/api/v1/widget/accounts/{slug}
Parser: json
Fetcher: plain
Pagination: one request per seed slug; the jobs envelope holds all jobs
Rate: 30 requests per minute
Concurrency: 1
Robots: allow public board endpoints; challenged or access-controlled boards are skipped
Attribution: Workable
Credentials: none
Slugs: example-co (seed placeholder)

## Behavior

- The account slug completes the URL and supplies the company name in normalized records.
- The jobs envelope holds postings. URLs come from url, then shortlink, with the account job path as the former fallback.
- Location text prefers the top-level city field, then the first entry of the locations array.
- Descriptions are not part of the widget payload used by the prior scraper, so normalized descriptions stay null.
- IDs, short codes, departments, remote flags, employment types, and dates are outside the normalized record contract.

## Quirks

- Boards without a configured seed are skipped; a search phrase is never used as a slug.
- The flat city, state, and country fields are preferred over the first nested location to match the prior parser.
- A board with no parseable jobs is treated as empty rather than an error.

## Break and fix log
- 2026-11-27: Live validation returned HTTP 200 with no records for the example slug. The source stays requires_slug and needs an active board seed.

- 2026-11-26: Ported the widget account JSON response with a slug manifest and synthetic fixture. The source stays requires_slug because an account slug must be supplied before a request can be built.
