# jobspresso

Source id: jobspresso
Name: Jobspresso
Family: feeds_api
Health: working
Endpoint: GET https://jobspresso.co/feed/?post_type=job_listing
Parser: rss
Fetcher: plain
Pagination: none
Profile: none
Credentials: none
Slugs: none
Rate: 20 requests per minute
Concurrency: 1
Robots: allow
Attribution: Jobspresso
Title separator: `": "`
Company: dc:creator when present, otherwise the title prefix before the colon with `company_first = true`; the title prefix wins when both are present

## Quirks

- Many titles use `Company: Title`; some are plain titles with the company in dc:creator. CDATA and entity names such as `AT&T` are supported.
- Links are job-listing pages with numeric ids.
- Descriptions are CDATA-wrapped text.

## Break and fix log
- 2026-11-27: Live validation returned HTTP 200 and records, but plain titles without a colon produced empty companies.
- 2026-11-27: Fixed by reading dc:creator from the RSS item as a company fallback when the title has no prefix; a CDATA value is cleaned before the first markup or entity marker. Verified via tier 1 replay and a live run that returned 20 records with companies such as Tailscale.

- 2026-11-26: Tier 1 manifest, synthetic six-file fixture, and offline replay test added.

