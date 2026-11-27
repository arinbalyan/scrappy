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
Company: Upstream-derived `company` field, required by the JSON parser

## Quirks

- The response is a bare JSON array. Public `role_description` is the description source; `city` supplies the location alias.
- Public posting URLs use `https://landing.jobs/at/<company-slug>/<job-slug>`, and the v1 feed carries no company name. The fixture adds the derived company and resolved URL.
- Offset and limit are substituted into the request query.
- The adapter requires an explicit URL; numeric ids in the feed cannot supply one.

## Break and fix log
- 2026-11-27: Live validation returned HTTP 200 but no records with title, company, and URL. The v1 API still returns a bare array with `title`, `role_description`, `city`, and `company_id`; it omits company names and returns numeric ids. Added description and location aliases and updated the fixture and golden to the public `/at/<company>/<job>` URL form.

- 2026-11-26: Tier 1 manifest, synthetic six-file fixture, and offline replay test added.

