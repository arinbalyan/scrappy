# wuzzuf

Source id: wuzzuf
Family: html_static
Health: working
Endpoint: GET https://wuzzuf.net/search/jobs/?q={query:term}
Parser: html with selectors
Fetcher: plain
Pagination: none
Rate: 10 requests per minute
Concurrency: 2
Robots: allow public search results
Attribution: Wuzzuf
Credentials: none
Slugs: none

## Behavior

- One search request returns static HTML; the old flow defaulted an empty term to software.
- The parser matches job-listing containers and reads the title and href from the job-title anchor, with company from a company-name span.
- The old flow paired global link and title matches by index and defaulted the company to Unknown Employer. The fixture uses aligned cards, so both flows produce the same records.

## Quirks

- /jobs/p/... URLs stay relative in normalized records.
- The old flow prefixing with the site base and its Wuzzuf Job title fallback are not reproduced by the parser.
- No browser or emulation profile is declared for the static search page.

## Break and fix log

- 2026-11-26: Manifest, static search fixture, behavior doc, and offline replay recorded during the Tier 2 port.

