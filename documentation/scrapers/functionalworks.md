# functionalworks

Source id: functionalworks
Family: feeds_api
Health: working
Endpoint: POST https://functional.works-hub.com/api/graphql
Parser: json
Fetcher: plain
Credentials: none
Rate: 20 requests per minute
Concurrency: 2
Robots: allow
Attribution: Functional Works

## Behavior and limitation

- The manifest issues the GraphQL POST query with a fixed page size, an application/json content type, and a declarative body template.
- Literal JSON braces in the body template use doubled braces, matching the adapter escape rule.
- The fixture response uses the GraphQL data.jobs envelope with nested company and location objects.
- The fixture carries nested company, location, remuneration, and tags fields; aliases read company.name, location.city, descriptionHtml, and an explicit URL.

## Known quirks

- Salary and tag-derived description text from the prior scraper are not part of the normalized record contract.

## Break and fix log

- 2026-11-27: Added the browser user-agent header used by the historical scraper. Live validation had returned HTTP 406 before the header change; replay fixtures and request templates were updated together.
- 2026-11-27: Live validation returned no records. The GraphQL endpoint and query shape remain unconfirmed; the `data.jobs` fixture matches the documented nested company, location, and `descriptionHtml` fields, so the manifest and fixture are unchanged.
- 2026-11-26: GET/JSON representation, generic fixtures, and offline replay recorded during the Tier 1 port.
- 2026-11-26: Added POST method, templated body, and header support to the adapter; replay now builds the GraphQL POST and health is working.
- 2026-11-26: Replaced the placeholder body with the full GraphQL query from the prior scraper and verified the frozen request bytes against the manifest template.
