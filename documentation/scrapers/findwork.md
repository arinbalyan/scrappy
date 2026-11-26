# findwork

Source id: findwork
Family: keyed
Health: keyed
Endpoint: GET https://findwork.dev/api/jobs/?search=...&location=...&sort_by=relevance with an authorization header
Parser: json
Fetcher: plain
Pagination: follows the next cursor link; the fixture ends with a null link
Rate: 170 requests per minute
Concurrency: 1
Robots: allow credentialed API use within published quotas
Attribution: FindWork
Credentials: findwork_auth (name only)
Slugs: none

## Quirks

- The credential is sent as an authorization header with a named placeholder. No value is stored in the manifest, and no committed example contains credential material.
- Jobs live under results; titles come from role, companies from company_name, and descriptions from text.
- A null location or description normalizes to null, as shown in the second golden record.
- The old scraper followed absolute or relative next links and deduplicated by job id; cursor iteration stays in the fetcher layer.

## Break and fix log

- 2026-11-26: Ported the credentialed API request with a synthetic response and golden records. The fixture value is a placeholder and carries no credential material.
- 2026-11-26: Corrected the request from an invented query parameter to the observed authorization header and added header rendering to the replay harness.
