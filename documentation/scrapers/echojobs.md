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
- 2026-11-27: Endpoint research returned no verified replacement for https://echojobs.io/api/jobs. The 429 is treated as a rate or environment challenge, so the manifest URL and health are unchanged. Re-probe from an unrestricted network with a low request rate before editing the endpoint.
- 2026-11-27: Live validation returned HTTP 429. The run environment may restrict outbound hosts; re-probe from an unrestricted network before changing health.

- 2026-11-26: Ported from historical behavior into the manifest adapter with synthetic fixtures.

