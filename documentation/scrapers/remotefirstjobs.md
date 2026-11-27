# remotefirstjobs

Source id: remotefirstjobs
Family: feeds_api
Health: working
Endpoint: GET https://remotefirstjobs.com/feed/
Parser: rss with title pattern `:`, company first
Fetcher: plain
Credentials: none
Rate: 20 requests per minute
Concurrency: 2
Robots: allow
Attribution: Remote First Jobs

## Quirks

- Title is encoded as `Company: Role`; the adapter splits on the colon.
- Remote-only metadata in the historical source is not represented in normalized fields.

## Break and fix log
- 2026-11-27: Live validation returned HTTP 403. The run environment may restrict outbound hosts; re-probe from an unrestricted network before changing health.

- 2026-11-26: Ported from historical behavior into the manifest adapter with synthetic fixtures.

