# echojobs

Source id: echojobs
Family: feeds_api
Health: working
Endpoint: GET https://echojobs.io/api/jobs
Parser: json with field aliases
Fetcher: plain
Credentials: none
Rate: 20 requests per minute
Concurrency: 2
Robots: allow
Attribution: EchoJobs

## Quirks

- Historical source accepted bare-array, `jobs`, and `data` shapes; this port covers one shape.
- Tags, remote flags, compensation, and dates are outside this adapter.

## Break and fix log

- 2026-11-26: Ported from historical behavior into the manifest adapter with synthetic fixtures.

