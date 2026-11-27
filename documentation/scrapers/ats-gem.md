# ats-gem

Source id: ats-gem
Family: ats
Health: requires_slug
Endpoint: GET https://api.gem.com/job_board/v0/{slug}/job_posts/
Parser: json
Fetcher: plain
Pagination: none; the board endpoint returns one complete list
Rate: 30 requests per minute
Concurrency: 1
Robots: allow public job board endpoints; boards without a published slug are skipped
Attribution: Gem
Credentials: none
Slugs: example-board (seed placeholder)

## Behavior

- The seed slug is the vanity URL path from the Gem job board settings and completes the request URL.
- The response is a flat JSON array; a company name is read from the synthetic `company` alias, then `team_display_name` or `organization`.
- The job URL comes from `absolute_url`, which the API returns as a complete hosted URL.
- Location text prefers `location.name`; descriptions prefer `plain_text_description`, then `description`.
- Departments, location type, employment type, IDs, and dates are outside the normalized record contract.

## Quirks

- Only published boards with a configured slug can produce records; a search phrase is never used as a slug.
- The documented Job Board API uses no authentication and no pagination. The batch GraphQL endpoint used by the prior scraper is not ported because it composes URLs from multiple response fields.
- Postings without a company alias are skipped by the parser, matching the normalized record contract.

## Break and fix log

- 2026-11-27: Ported the public REST job-board list with a slug manifest and synthetic fixture. The source stays requires_slug because a board vanity path must be supplied before a request can be built.

## Live seed verification

- 2026-11-27: verified live seed `coupa-software-inc-ats-1` through the manifest adapter, returning 300 records.
- The manifest now ships this verified seed and is declared working. Re-probe when the tenant board moves.
