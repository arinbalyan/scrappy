# reed

Source id: reed
Family: html_static
Health: working
Endpoint: GET https://www.reed.co.uk/jobs?keywords={query:keywords}&location={query:location}&pageno={query:pageno}
Parser: json
Fetcher: plain
Pagination: numbered pages with 200-500 ms jitter and up to three 429 retries
Rate: 20 requests per minute
Concurrency: 2
Robots: allow public job search
Attribution: Reed
Credentials: none in this path; the old API used a separate REED_API_KEY
Slugs: none

## Behavior

- The response is an HTML page with a __NEXT_DATA__ script; the parser extracts the JSON envelope at props.pageProps.searchResults.jobs.
- Title, company, location, and description come from nested jobDetail fields.
- Company falls back from ouName to profileName.
- The historical scraper resolved relative URLs against https://www.reed.co.uk; the fixture supplies absolute resolvedUrl values so the adapter can reproduce them.
- Salary, posting date, remote status, and job type are outside the six-field normalized record.

## Known quirks

- Items with an empty title or a zero job id are discarded before normalization.
- The old page regex tolerated attribute order variations; the embedded parser expects the quoted id attribute form used in the fixture.
- The keyed API route is represented by a credential name only; no value appears here.

## Break and fix log

- 2026-11-26: Manifest, synthetic fixtures, behavior doc, and offline replay recorded during the Tier 2 B port.
