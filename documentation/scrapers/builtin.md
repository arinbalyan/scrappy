# builtin

Source id: builtin
Family: html_static
Health: working
Endpoint: GET https://builtin.com/jobs?search={query:search}
Parser: html
Fetcher: plain
Pagination: page zero through five; the frozen fixture uses page zero
Rate: 20 requests per minute
Concurrency: 2
Robots: allow public job pages
Attribution: Built In
Credentials: none
Slugs: none

## Behavior

- The response is static HTML with job cards marked by `data-id="job-card"`.
- Title, company, and URL come from `data-id` attributes on links inside each card.
- URLs are relative in live HTML and remain relative in normalized records.
- The page also carries a JSON-LD `ItemList`, but the HTML cards provide the same records without entity-encoding handling.

## Quirks

- Location and description are not extracted by the declarative selector parser.
- An anti-bot challenge response is treated as a fetch failure by the caller; the manifest declares no challenge handling.

## Break and fix log
- 2026-11-27: Live validation returned HTTP 200 but no __NEXT_DATA__ blob. The page now renders job cards with stable `data-id` attributes; switched the manifest and fixture to the HTML parser and verified replay.
- 2026-11-27: Added the browser user-agent header used by the historical scraper after a live probe returned HTTP 200 with a challenged or variant page. Replay fixtures were updated to match.
- 2026-11-27: Live validation returned HTTP 200 but no embedded JSON source was found. Public pages continue to carry a __NEXT_DATA__ blob with jobs under props.pageProps.jobs, so the missing blob likely came from a challenged or query-free variant. The manifest and fixture keep that path; a response without the blob still fails replay.

- 2026-11-26: Manifest, embedded-JSON fixture, behavior doc, and offline replay recorded during the Tier 2 port.

