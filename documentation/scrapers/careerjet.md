# careerjet

Source id: careerjet
Family: keyed
Health: keyed
Endpoint: GET https://public.api.careerjet.net/search?affid=...&locale_code=...&keywords=...&sort=date&page=...&pagesize=...
Parser: json
Fetcher: plain
Pagination: numbered pages up to 10; a short page or the last page ends the run
Rate: 170 requests per minute
Concurrency: 1
Robots: allow credentialed API use within the affiliate quota
Attribution: Careerjet
Credentials: careerjet_affid (name only)
Slugs: none

## Quirks

- Jobs live under jobs; a job without a title or URL is skipped.
- Location is a single locations string and feeds the location field directly.
- The old scraper deduplicated by URL and converted salary codes to intervals; those steps are outside the declarative parser.
- The old client throttled pages to roughly three requests per second; the manifest rate cap reflects that ceiling.

## Break and fix log

- 2026-11-26: Ported the affiliate API request with a named credential placeholder and a synthetic response. Samples in the fixture are not live credentials.

