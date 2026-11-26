# deel

Source id: deel
Family: feeds_api
Health: keyed
Endpoint: GET https://api.letsdeel.com/rest/v2/ats/job-postings?limit=25
Parser: JSON with field aliases
Fetcher: plain
Credentials: DEEL_API_TOKEN
Rate: 20 requests per minute
Concurrency: 2
Robots: allow
Attribution: Deel

## Quirks

- Historical source required DEEL_API_TOKEN and read postings from data. Company is derived from department; salary and dates are outside this adapter.

## Break and fix log

- 2026-11-26: Ported from historical behavior into the manifest adapter with synthetic fixtures.
- 2026-11-26: Live probe returned HTTP 401, consistent with the declared DEEL_API_TOKEN requirement.
