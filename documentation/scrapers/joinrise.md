# joinrise

Source id: joinrise
Family: feeds_api
Health: working
Endpoint: GET https://api.joinrise.ai/api/job/search?limit=25
Parser: JSON with field aliases
Fetcher: plain
Credentials: none
Rate: 20 requests per minute
Concurrency: 2
Robots: allow
Attribution: Joinrise

## Quirks

- Historical source read jobs from result.jobs with nested owner and breakdown fields; this port uses dotted aliases. Client-side filtering, compensation, and remote detection are outside this adapter.

## Break and fix log
- 2026-11-27: Endpoint research returned no verified replacement for the public search URL. The probe failure was a host resolution error, so the manifest URL and working health stay unchanged. Re-probe from an unrestricted network before changing the endpoint.
- 2026-11-27: Live validation could not reach the public search endpoint. Re-probe from an unrestricted network before changing health.

- 2026-11-26: Ported from historical behavior into the manifest adapter with synthetic fixtures.
- 2026-11-26: Live probe could not resolve the API host from this network; endpoint needs a retry from an unrestricted network.
