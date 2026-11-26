# weworkremotely

Source id: weworkremotely
Family: feeds_api
Health: working
Endpoint: GET https://weworkremotely.com/remote-jobs.rss
Parser: rss
Fetcher: plain
Credentials: none
Rate: 20 requests per minute
Concurrency: 2
Robots: allow
Attribution: We Work Remotely

## Behavior

- Single unauthenticated GET returns an RSS channel.
- Item titles use a "Company: Title" shape; parser options split on ": " with the company first.
- Region, country, state, and skills fields are observed in the feed but are not part of the normalized record; only title, company, link, and description survive parsing.
- Client-side search-term filtering existed in the prior scraper and is not part of the adapter contract.

## Known quirks

- A title without the separator leaves the whole title as the job title and an empty company.
- The parser derives no location from the custom region and country tags.

## Break and fix log

- 2026-11-26: Manifest, generic fixtures, and offline replay recorded during the Tier 1 port.
