# usajobs

Source id: usajobs
Family: keyed
Health: keyed
Endpoint: GET https://data.usajobs.gov/api/Search?results_per_page=25&page=1
Parser: JSON with field aliases
Fetcher: plain
Credentials: USAJOBS_API_KEY, USAJOBS_EMAIL
Rate: 20 requests per minute
Concurrency: 2
Robots: allow
Attribution: USAJobs

## Quirks

- Historical source required Authorization-Key and a contact email User-Agent, and joined summary and duties; only the summary alias is carried here.

## Break and fix log

- 2026-11-26: Ported from historical behavior into the manifest adapter with synthetic fixtures.
- 2026-11-26: Live probe returned an HTTP 403 Akamai access denial, so health changed to blocked.
- 2026-11-27: Live validation could not build the request without USAJOBS_API_KEY and USAJOBS_EMAIL, so keyed is the accurate pre-credential state. The earlier 403 remains a deployment risk and must be re-probed with real credentials.
