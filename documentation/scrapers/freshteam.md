# freshteam

Source id: freshteam
Family: keyed
Health: keyed
Endpoint: GET https://{slug}.freshteam.com/api/job_postings
Parser: JSON root array
Fetcher: plain
Credentials: FRESHTEAM_API_TOKEN
Slugs: example-co (seed placeholder)

## Behavior

- The job posting endpoint requires a tenant slug in the host and a bearer token in the Authorization header.
- Titles come from title and descriptions from description.
- The URL comes from applicant_apply_link, with url and apply_url as ordered fallbacks.
- Location comes from branch, with location and city as fallbacks.
- Company is taken from an explicit feed field, and URL values are explicit in fixtures because the old code derived them from the seed.

## Quirks

- The old scraper accepted a slug:key seed and synthesized a job URL when applicant_apply_link was empty. Seed splitting and URL synthesis are outside the adapter, so fixtures carry explicit apply links and companies.
- A search phrase was rejected as a tenant seed by the old scraper. The manifest only substitutes a configured slug and never treats a search phrase as one.
- The old scraper normalized employment types, set remote flags, read dates, and built identifiers. Normalization and derived fields are outside the normalized record contract.

## Break and fix log

- 2026-11-26: Ported from historical behavior with a keyed manifest and synthetic fixture values.
- 2026-11-26: Replay uses a synthetic token and explicit apply links; health stays keyed because the token is caller-supplied.
