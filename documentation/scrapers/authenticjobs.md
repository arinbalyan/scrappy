# authenticjobs

Source id: authenticjobs
Family: keyed
Health: keyed
Endpoint: GET https://authenticjobs.com/api/?api_key={key:AUTHENTICJOBS_API_KEY}&page=1
Parser: JSON with field aliases
Fetcher: plain
Credentials: AUTHENTICJOBS_API_KEY
Rate: 20 requests per minute
Concurrency: 2
Robots: allow
Attribution: AuthenticJobs

## Quirks

- Historical source sent api_key and page query parameters and synthesized a job URL from the listing id; URL synthesis is outside the adapter.

## Break and fix log

- 2026-11-26: Ported from historical behavior into the manifest adapter with synthetic fixtures.
- 2026-11-26: Live probe returned an HTTP 403 Cloudflare challenge, so health changed to blocked.
