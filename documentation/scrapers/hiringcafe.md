# hiringcafe

Source id: hiringcafe
Family: html_static
Health: working
Endpoint: GET https://hiring.cafe/
Parser: json
Fetcher: plain
Pagination: none, one homepage request
Rate: 20 requests per minute
Concurrency: 2
Robots: allow public job listings
Attribution: Hiring Cafe
Credentials: none
Slugs: none

## Behavior

- The response is the homepage HTML with a __NEXT_DATA__ script; the parser extracts props.pageProps.ssrHits.
- Title comes from job_information.title, with job_title_raw as the fallback.
- Company comes from enriched_company_data.name and the normalized URL from apply_url.
- The old scraper capped output at the requested count; the adapter emits every hit in the frozen payload.
- Hits with no title or no apply URL are discarded before normalization.

## Known quirks

- Location and description are absent from the SSR hit shape and normalize to null.
- The old scraper used an id hash; the six-field record keeps only the apply URL.

## Break and fix log

- 2026-11-26: Manifest, synthetic fixtures, behavior doc, and offline replay recorded during the Tier 2 B port.
