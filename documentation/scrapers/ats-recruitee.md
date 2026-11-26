# ats-recruitee

Source id: ats-recruitee
Family: ats
Health: requires_slug
Endpoint: GET https://{slug}.recruitee.com/api/offers
Parser: json
Fetcher: plain
Pagination: one request per seed slug; the offers envelope holds all offers
Rate: 30 requests per minute
Concurrency: 1
Robots: allow public board endpoints; challenged or access-controlled boards are skipped
Attribution: Recruitee
Credentials: none
Slugs: example-co (seed placeholder)

## Behavior

- The seed slug selects the subdomain and supplies the company name in normalized records.
- Offers live in the offers envelope.
- URLs come from careers_url, with the reconstructed offer path as the former fallback when it is absent.
- Location text is read from city, then state, then country.
- Descriptions are carried as provided; the prior scraper stripped HTML tags after trimming.
- IDs, departments, remote flags, salary fields, and dates are outside the normalized record contract.

## Quirks

- Boards without a configured seed are skipped; a search phrase is never used as a slug.
- Offers in the fixture use absolute URLs so replay preserves the careers_url form.
- A board with no parseable offers is treated as empty rather than an error.

## Break and fix log

- 2026-11-26: Ported the offers JSON response with a slug manifest and synthetic fixture. The source stays requires_slug because a board slug must be supplied before a request can be built.
