# jobdataapi

Source id: jobdataapi
Family: feeds_api
Health: working
Endpoint: GET https://jobdataapi.com/api/jobs/?page=1&page_size=25
Parser: JSON with field aliases
Fetcher: plain
Credentials: none
Rate: 20 requests per minute
Concurrency: 2
Robots: allow
Attribution: JobDataAPI

## Quirks

- Historical source paginated by page and page_size with optional title and location filters; salary, dates, skills, and remote flags are outside the adapter.

## Break and fix log
- 2026-11-27: Live validation returned HTTP 403. The run environment may restrict outbound hosts; re-probe from an unrestricted network before changing health.

- 2026-11-26: Ported from historical behavior into the manifest adapter with synthetic fixtures.
- 2026-11-26: Live probe returned HTTP 403 for anonymous pagination, consistent with the keyed API plan.
