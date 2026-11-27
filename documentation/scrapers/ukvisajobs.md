# ukvisajobs

Source id: ukvisajobs
Family: feeds_api
Health: working
Endpoint: GET https://www.ukvisajobs.com/
Parser: json
Fetcher: plain
Pagination: none
Rate: 12 requests per minute
Concurrency: 1
Robots: allow
Attribution: UK Visa Jobs
Credentials: none
Slugs: none

## Behavior

- One GET returns the public board. No browser or emulation profile is used.
- The body is HTML with a `__NEXT_DATA__` JSON element. The parser reads `props.pageProps.result.experienced_jobs` and maps `title`, `company_name`, `job_link`, `city`, and `description`.
- Records with a missing or blank title or URL are dropped; a missing company becomes "unknown".
- The prior scraper retried the same body as static HTML when JSON decoding failed. The manifest records the embedded JSON path.

## Known quirks

- The static HTML fallback matched columns by array position and used one shared apply URL, so it is not represented in the normalized fixture.
- Salary and degree details from the HTML fallback are outside the normalized record fields.

## Break and fix log
- 2026-11-27: Live validation returned HTTP 200 but invalid JSON. Public job data also appears behind `https://my.ukvisajobs.com/ukvisa-api/api/fetch-jobs-data` with a token and cookies. The exact private request shape is not public; the manifest uses the public `__NEXT_DATA__` payload instead.

- 2026-11-27: Switched the parser from plain JSON to embedded JSON in the `__NEXT_DATA__` element, using `props.pageProps.result.experienced_jobs` as the envelope. Added aliases for `company_name`, `job_link`, `city`, and `description` and updated the fixture to the HTML response with the embedded payload.

- 2026-11-26: Manifest, synthetic JSON fixture, behavior doc, and offline replay recorded during the Tier 2 C port.
