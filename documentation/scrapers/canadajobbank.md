# canadajobbank

Source id: canadajobbank
Family: feeds_api
Health: working
Endpoint: GET https://jobbank.api.canada.ca/api/job?resource_id=ra-gj-bd&limit=25
Parser: JSON with field aliases
Fetcher: plain
Credentials: none
Rate: 20 requests per minute
Concurrency: 2
Robots: allow
Attribution: Canada Job Bank

## Quirks

- Historical source checked the success flag and read records from result.records; this port reads that envelope. Search term, salary, and date fields are outside this adapter.

## Break and fix log
- 2026-11-27: Live validation could not reach the public job bank endpoint. Re-probe from an unrestricted network before changing health.

- 2026-11-26: Ported from historical behavior into the manifest adapter with synthetic fixtures.
- 2026-11-26: Live probe could not resolve the API host from this network; endpoint needs a retry from an unrestricted network.
