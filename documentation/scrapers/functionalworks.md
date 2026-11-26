# functionalworks

Source id: functionalworks
Family: feeds_api
Health: stale
Endpoint: GET https://functional.works-hub.com/api/graphql
Parser: json
Fetcher: plain
Credentials: none
Rate: 20 requests per minute
Concurrency: 2
Robots: allow
Attribution: Functional Works

## Behavior and limitation

- The prior scraper issued a POST GraphQL query with a fixed page size. The Request type in this crate exposes only GET, and the plain fetcher has no POST path, so the POST contract cannot be ported.
- The manifest and fixtures replay only the observable GET/simple JSON representation: a GraphQL-shaped data.jobs envelope reached through a GET URL.
- The fixture carries nested company, location, remuneration, and tags fields; aliases read company.name, location.city, descriptionHtml, and an explicit URL.
- Health is stale because the live path requires a POST that this adapter does not implement.

## Known quirks

- No request body can be replayed from the manifest.
- Salary and tag-derived description text from the prior scraper are not part of the normalized record contract.

## Break and fix log

- 2026-11-26: GET/JSON representation, generic fixtures, and offline replay recorded during the Tier 1 port; POST remains unported.
