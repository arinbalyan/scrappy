# hiringthing

Source id: hiringthing
Family: keyed
Health: keyed
Endpoint: GET https://api.hiringthing.com/api/v1/jobs
Parser: JSON with jobs envelope
Fetcher: plain
Credentials: HIRINGTHING_API_TOKEN
Rate: 30 requests per minute

## Behavior

- The job feed returns a jobs envelope with one record per posting and requires a bearer token.
- Titles come from title and URL from url, with apply_url as the second choice.
- Location comes from the flat location string; company from company_name.
- Fixtures carry explicit URLs because the historical derived URL is outside the adapter.

## Quirks

- The historical scraper used the search term as a tenant seed for company names and IDs. The manifest cannot synthesize fields, so fixtures carry resolved company values.
- Dates, departments, types, and IDs are outside the normalized record contract.

## Break and fix log

- 2026-11-26: Ported from historical behavior with a manifest and synthetic fixture.
- 2026-11-26: Live probe returned HTTP 401 with an invalid-credentials message; health changed to keyed.

