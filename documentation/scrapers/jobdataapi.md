# jobdataapi

Source id: jobdataapi
Family: feeds_api
Health: working
Endpoint: GET https://jobdataapi.com/api/jobs/
Parser: JSON with field aliases
Fetcher: plain
Credentials: none
Rate: 20 requests per minute
Concurrency: 2
Robots: allow
Attribution: JobDataAPI

## Quirks

- Historical source paginated by page and page_size; anonymous requests now reject pagination, so the manifest uses the bare endpoint.
- Salary, dates, skills, and remote flags are outside the adapter.

## Break and fix log
- 2026-11-27: Live validation returned HTTP 403 because anonymous requests cannot paginate. Switched to the bare endpoint, which returned 20 records.

- 2026-11-26: Ported from historical behavior into the manifest adapter with synthetic fixtures.
- 2026-11-26: Live probe returned HTTP 403 for anonymous pagination, consistent with the keyed API plan.
