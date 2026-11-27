# landingjobs

Source id: landingjobs
Name: Landingjobs
Family: feeds_api
Health: working
Endpoint: GET https://landing.jobs/api/v1/jobs?offset={query:offset}&limit={query:limit}
Parser: json
Fetcher: plain
Pagination: none (query offsets are supported; page orchestration is outside the parser adapter)
Profile: none
Credentials: none
Slugs: none
Rate: 20 requests per minute
Concurrency: 1
Robots: allow
Attribution: Landingjobs
Company: Top-level `company` field, required by the JSON parser

## Quirks

- The response is a bare JSON array.
- Offset and limit are substituted into the request query.
- Numeric ids in links are retained as normalized URLs.

## Break and fix log
- 2026-11-27: Live validation returned HTTP 200 but no records with title, company, and URL. The HTML aliases need review against the live payload.

- 2026-11-26: Tier 1 manifest, synthetic six-file fixture, and offline replay test added.

