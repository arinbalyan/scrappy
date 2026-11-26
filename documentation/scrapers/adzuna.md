# adzuna

Source id: adzuna
Family: keyed
Health: keyed
Endpoint: GET https://api.adzuna.com/v1/api/jobs/{country}/search/{page}?app_id=...&app_key=...&results_per_page=...&sort_by=date
Parser: json
Fetcher: plain
Pagination: numbered pages up to 20; a short page ends the run
Rate: 60 requests per minute
Concurrency: 1
Robots: allow credentialed API use within published quotas
Attribution: Adzuna
Credentials: adzuna_app_id, adzuna_app_key (names only)
Slugs: none

## Quirks

- Company and location use nested display_name fields addressed by dotted aliases.
- The response wraps jobs in results; descriptions and redirect URLs are kept as provided.
- The old scraper stripped description tags and skipped predicted salaries and duplicate ids; those post-processing steps are outside the declarative parser and are not reproduced in the fixture.
- Missing credentials skip the source before any request is sent.

## Break and fix log

- 2026-11-26: Ported the credentialed API request with named placeholders and a synthetic response. Values in the fixture are samples and carry no credential material.

