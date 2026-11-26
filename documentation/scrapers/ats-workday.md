# ats-workday

Source id: ats-workday
Family: ats
Health: requires_slug
Endpoint: POST https://{slug}.wd5.myworkdayjobs.com/wday/cxs/{slug}/External/jobs
Parser: JSON with jobPostings envelope
Fetcher: plain
Rate: 30 requests per minute
Concurrency: 1

## Behavior

- The seed supplies both the tenant subdomain and site path in the URL.
- The request posts a fixed JSON body with limit 20, offset 0, and empty search text.
- The jobPostings envelope holds postings. Fixtures carry explicit URLs and company values because the historical code composed them from externalPath and the tenant.
- Location comes from locationsText; company comes from the slug.

## Quirks

- Multi-token seeds for alternate WD numbers and sites stay outside the manifest; the default is wd5 and External.
- Offset pagination and regex extraction of job IDs stay outside the adapter.

## Break and fix log

- 2026-11-26: Ported the tenant job search POST with a slug manifest and synthetic fixture.
- 2026-11-26: Live POST probe of a public tenant returned HTTP 200 with 342 job postings; health left at requires_slug because the tenant and site are supplied by the caller.

