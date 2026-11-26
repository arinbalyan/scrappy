# simplyhired

Source id: simplyhired
Family: html_static
Health: working
Endpoint: GET https://www.simplyhired.com/search?q={query:q}&l={query:l}&pn={query:pn}
Parser: html
Fetcher: plain
Pagination: numbered pages, up to 3, stopping when a page yields no records
Rate: 12 requests per minute
Concurrency: 1
Robots: allow
Attribution: Simply Hired
Credentials: none
Slugs: none

## Behavior

- A single GET returns a static search results page. No browser or emulation profile is used.
- Each job card is selected by its data-testid marker. Title, company, and URL come from elements inside the card.
- The card hrefs are absolute in the fixture, matching the normalized URLs produced by the prior scraper. The selector parser emits hrefs as written.
- Navigation anchors outside the cards do not match the item selector.
- The prior scraper also tried an embedded Next.js blob, then JSON-LD, when cards were absent. The declarative HTML path covers the card layout recorded in the fixture.

## Known quirks

- Card markup depends on data-testid attributes remaining in the rendered HTML.
- Location and description snippets are ignored by the selector-based normalization.

## Break and fix log

- 2026-11-26: Manifest, synthetic HTML fixture, behavior doc, and offline replay recorded during the Tier 2 C port.
