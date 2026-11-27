# ecojobs

Source id: ecojobs
Family: feeds_api
Health: working
Endpoint: GET https://www.ecojobs.com/rss.xml
Parser: rss
Fetcher: plain
Credentials: none
Rate: 20 requests per minute
Concurrency: 2
Robots: allow
Attribution: Eco Jobs

## Behavior

- Single unauthenticated GET returns an RSS channel.
- The /rss.xml path serves a sitemap-shaped RSS document with job links and no descriptions.
- The /feed/ path serves editorial articles and is not used by this adapter.
- Both CDATA-wrapped and plain item descriptions parse into normalized records.
- The parser derives the company from the link host.

## Known quirks

- No software-specific filter is applied in the adapter; filtering belongs to callers.
- The sitemap feed has no descriptions, so downstream matching must rely on titles and links.
- Slug-based ids from the prior scraper are not part of the normalized record contract.

## Break and fix log
- 2026-11-27: Live validation returned 50 RSS records from /rss.xml, so health is working. The /feed/ path was tested and rejected because it carries editorial articles.
- 2026-11-27: Live validation confirmed the /rss.xml path still serves job links. Endpoint research found no replacement.
- 2026-11-26: Manifest, generic fixtures, and offline replay recorded during the Tier 1 port.
