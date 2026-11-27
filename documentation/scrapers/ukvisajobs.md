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
- The primary path decodes a JSON array of job objects with title, company, URL, location, and description fields.
- Records with a missing or blank title or company are dropped.
- The prior scraper retried the same body as static HTML when JSON decoding failed. The manifest records the JSON array path.

## Known quirks

- The static HTML fallback matched columns by array position and used one shared apply URL, so it is not represented in the normalized fixture.
- Salary and degree details from the HTML fallback are outside the normalized record fields.

## Break and fix log
- 2026-11-27: Live validation returned HTTP 200 but invalid JSON. Public job data now appears behind `https://my.ukvisajobs.com/ukvisa-api/api/fetch-jobs-data` with a token and cookies and a `job_link` field. The root URL serves HTML, which explains the parse failure. The exact request shape is not public, so the manifest and fixture are unchanged.

- 2026-11-26: Manifest, synthetic JSON fixture, behavior doc, and offline replay recorded during the Tier 2 C port.
