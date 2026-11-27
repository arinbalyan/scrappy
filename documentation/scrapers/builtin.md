# builtin

Source id: builtin
Family: html_static
Health: working
Endpoint: GET https://builtin.com/jobs?search={query:search}
Parser: json with embedded __NEXT_DATA__
Fetcher: plain
Pagination: page zero through five; the frozen fixture uses page zero
Rate: 20 requests per minute
Concurrency: 2
Robots: allow public job pages
Attribution: Built In
Credentials: none
Slugs: none

## Behavior

- The response is static HTML carrying a JSON blob in the __NEXT_DATA__ script element.
- The parser selects the props.pageProps.jobs array and maps title, company_name, url, city_name, and body_teaser.
- The old flow built a URL from the alias field when the blob had no url. The adapter requires a url on every record, so the fixture uses an absolute url.
- The Alpine and data-id HTML fallback and the JSON-LD fallback are not represented by one declarative parser; a response without the embedded blob fails replay instead of falling back.

## Quirks

- Relative URLs stay relative in normalized records.
- Descriptions keep the source markup, including inline tags.
- An anti-bot challenge response is treated as a fetch failure by the caller; the manifest declares no challenge handling.

## Break and fix log
- 2026-11-27: Live validation returned HTTP 200 but no embedded JSON source was found. The page markup or embedded envelope needs review.

- 2026-11-26: Manifest, embedded-JSON fixture, behavior doc, and offline replay recorded during the Tier 2 port.

