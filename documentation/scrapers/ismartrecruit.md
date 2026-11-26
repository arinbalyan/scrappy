# ismartrecruit

Source id: ismartrecruit
Family: keyed
Health: keyed
Endpoint: GET https://app.ismartrecruit.com/WebsiteJSONAPI?apiKey={key:ISMARTRECRUIT_API_KEY}&jobTitle={query:job_title}&city={query:city}&start=0&numOfRecords=25
Parser: JSON root array
Fetcher: plain
Credentials: ISMARTRECRUIT_API_KEY
Queries: job_title, city

## Behavior

- The API key is passed as a query parameter and is required before a request can be built.
- Titles come from jobTitle and company from companyName, falling back to a declared company field in fixtures.
- URL prefers applyUrl; the historical derived description URL is carried explicitly in fixtures.
- Location prefers city, then country.

## Quirks

- The historical scraper filtered by search term in the client. The adapter does not apply client-side filters.
- Remote detection, dates, categories, and IDs are outside the normalized record contract.

## Break and fix log

- 2026-11-26: Ported from historical behavior with a keyed manifest and synthetic fixture.
- 2026-11-26: Live probe returned HTTP 200 with a base64 error payload stating that no API key was found.

