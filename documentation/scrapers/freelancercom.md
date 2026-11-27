# freelancercom

Source id: freelancercom
Family: feeds_api
Health: working
Endpoint: GET https://www.freelancer.com/api/projects/0.1/projects/active
Parser: json
Fetcher: plain
Pagination: offset within the first fifty active projects
Rate: 20 requests per minute
Concurrency: 1
Robots: allow public project API
Attribution: Freelancer.com
Credentials: none
Slugs: none

## Behavior

- The fixture request adds compact, limit, full_description, and query parameters.
- Projects sit under result.projects; the status field is not used by the parser.
- Title, description, owner display name, city, and seo_url are mapped to normalized fields.
- The old flow built a project URL from seo_url or the numeric id. The fixture carries seo_url values, so no synthetic id-based URL is needed.

## Quirks

- seo_url values are slugs, so golden URLs are relative slugs for this replay.
- Budget, currency, type, and submitted time are dropped by the six-field normalized contract.
- Compensation and posting date never appear in fixtures or docs.

## Break and fix log
- 2026-11-27: Live validation returned HTTP 200 but no records with title, company, and URL. The HTML aliases need review against the live payload.

- 2026-11-26: Manifest, JSON API fixture, behavior doc, and offline replay recorded during the Tier 2 port.

