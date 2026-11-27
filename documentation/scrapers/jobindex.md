# jobindex

Source id: jobindex
Family: html_static
Health: working
Endpoint: GET https://www.jobindex.dk/jobsoegning?q={query:term}
Parser: json with embedded Stash variable
Fetcher: plain
Pagination: none
Rate: 20 requests per minute
Concurrency: 2
Robots: allow public search results
Attribution: Jobindex
Credentials: none
Slugs: none

## Behavior

- The search page embeds a Stash variable holding the result payload; the parser reads it by variable name.
- Results sit under the nested jobsearch.result_app.storeData.searchResponse.results path.
- headline, companytext, share_url, area, and html map to title, company, url, location, and description.
- The old flow first tried a direct JSON array response and fell back to Stash. A single declarative parser selects the Stash path, so a plain JSON array response is not replayable through this manifest.

## Quirks

- The embedded description keeps its markup; the old flow stripped tags before storing text.
- First date, tid, and missing-field defaults are dropped by the six-field normalized contract.
- Missing company or title rows are skipped by the parser.

## Break and fix log
- 2026-11-27: Live validation returned HTTP 204 with no embedded JSON source. The page endpoint or markup needs review.

- 2026-11-26: Manifest, embedded Stash fixture, behavior doc, and offline replay recorded during the Tier 2 port.

