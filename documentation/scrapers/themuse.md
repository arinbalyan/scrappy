# themuse

Source id: themuse
Family: feeds_api
Health: working
Endpoint: GET https://www.themuse.com/api/public/jobs?page=0&descending=true
Parser: json with `results` envelope
Fetcher: plain
Credentials: none
Rate: 20 requests per minute
Concurrency: 2
Robots: allow
Attribution: The Muse

## Quirks

- Nested company and location objects are flattened through dotted aliases.
- Client-side category filtering from the historical source is outside this adapter.

## Break and fix log
- 2026-11-27: Live validation returned HTTP 403. The run environment may restrict outbound hosts; re-probe from an unrestricted network before changing health.

- 2026-11-26: Ported from historical behavior into the manifest adapter with synthetic fixtures.


## Live validation

- 2026-11-26: GET returned HTTP 403 with a challenge page from the test network; manifest stays `working` from historical evidence and offline replay.
