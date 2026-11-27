# crunchboard

Source id: crunchboard
Name: Crunchboard
Family: feeds_api
Health: working
Endpoint: GET https://www.crunchboard.com/jobs.rss
Parser: rss
Fetcher: plain
Pagination: none
Profile: none
Credentials: none
Slugs: none
Rate: 20 requests per minute
Concurrency: 1
Robots: allow
Attribution: Crunchboard
Title separator: `" - "`
Company: First title segment; titles use `Company - Title` with `company_first = false`

## Quirks

- Feed company fields are empty; titles use `Company - Title`.
- The title prefix supplies a non-empty company.
- Descriptions are CDATA-wrapped text.

## Break and fix log
- 2026-11-27: Endpoint research returned no verified replacement for https://www.crunchboard.com/jobs.rss. The 403 is treated as a challenge, so the manifest URL and health stay as recorded. Re-probe with a browser User-Agent from an unrestricted network before changing the endpoint.
- 2026-11-27: Live validation returned HTTP 403. The run environment may restrict outbound hosts; re-probe from an unrestricted network before changing health.

- 2026-11-26: Tier 1 manifest, synthetic six-file fixture, and offline replay test added.


## Live relabel 2026-11-27

- Live probe observed the endpoint as `blocked`. The manifest health was corrected to match the observed outcome.
