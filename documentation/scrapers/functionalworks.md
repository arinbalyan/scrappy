# functionalworks

Source id: functionalworks
Family: html_static
Health: working
Endpoint: GET https://works-hub.com/jobs
Parser: html
Fetcher: plain
Pagination: none; the public board renders all roles in one page
Rate: 20 requests per minute
Concurrency: 2
Robots: allow public job pages
Attribution: Functional Works
Credentials: none
Slugs: none

## Behavior

- The public board redirects from `functional.works-hub.com/jobs` to `works-hub.com/jobs`.
- Job cards are anchors with `href="/jobs/..."`, a `text-coral` company label, and an `h3` title.
- The HTML parser reads the card anchor as the URL, the `h3` as the title, and the company label as the company.
- Relative URLs stay relative in normalized records, matching the parser contract.
- The fixture is synthetic and mirrors the card markup without copying live content.

## Quirks

- Location and description are not available from the declarative selector parser.
- The page reports 259 roles, but only the first page of cards is rendered without client-side pagination.

## Break and fix log
- 2026-11-27: The latest live sweep recorded functionalworks in the working
  section with 259 records. Keep the html_static board path when reconciling
  status with that sweep.
- 2026-11-27: The GraphQL entries below describe superseded attempts. The
  html_static board path is the current manifest and fixture configuration.
- 2026-11-27: Live validation showed the GraphQL endpoint returns HTTP 406 for HTML-only clients. The public board now renders jobs server-side on `works-hub.com/jobs`, so the manifest switched from `feeds_api` to `html_static` and the fixture, status, and replay were updated.
- 2026-11-27: Added the browser user-agent header used by the historical scraper. Live validation had returned HTTP 406 before the header change; replay fixtures and request templates were updated together.
- 2026-11-27: Live validation returned no records. The GraphQL endpoint and query shape remain unconfirmed; the `data.jobs` fixture matches the documented nested company, location, and `descriptionHtml` fields, so the manifest and fixture were unchanged.
- 2026-11-26: GET/JSON representation, generic fixtures, and offline replay recorded during the Tier 1 port.
- 2026-11-26: Added POST method, templated body, and header support to the adapter; replay now builds the GraphQL POST and health is working.
- 2026-11-26: Replaced the placeholder body with the full GraphQL query from the prior scraper and verified the frozen request bytes against the manifest template.

