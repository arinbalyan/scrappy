# infojobs

Source id: infojobs
Family: keyed
Health: keyed
Endpoint: GET https://api.infojobs.net/api/7/offer?page=1
Parser: JSON with field aliases
Fetcher: plain
Credentials: INFOJOBS_CLIENT_ID, INFOJOBS_CLIENT_SECRET, INFOJOBS_BASIC_TOKEN
Rate: 20 requests per minute
Concurrency: 2
Robots: allow
Attribution: InfoJobs

## Quirks

- Historical source derived a Basic token from client ID and secret and extracted emails from descriptions; token derivation and email extraction are outside the adapter.

## Break and fix log

- 2026-11-26: Ported from historical behavior into the manifest adapter with synthetic fixtures.
- 2026-11-26: Live probe returned HTTP 401, consistent with the declared Basic credentials.
