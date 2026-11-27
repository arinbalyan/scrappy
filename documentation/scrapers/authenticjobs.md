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
- 2026-11-27: Live validation could not build the request without AUTHENTICJOBS_API_KEY, so keyed is the accurate pre-credential state. The earlier 403 remains a deployment risk and must be re-probed with a real key.
