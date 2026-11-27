# drupaljobs

Source id: drupaljobs
Family: feeds_api
Health: stale
Endpoint: GET https://jobs.drupal.org/filtered-jobs/%25/%25/%25/%25/%25/feed
Parser: rss
Fetcher: plain
Credentials: none
Rate: 20 requests per minute
Concurrency: 2
Robots: allow
Attribution: Drupal Jobs

## Quirks

- The endpoint now returns an HTML shutdown notice, not an RSS feed.
- Drupal announced that jobs.drupal.org stopped being available in September 2026.
- The historical wildcard path and company-from-host fallback no longer yield records.

## Break and fix log
- 2026-11-27: Live validation returned HTTP 200 with an HTML shutdown notice that states jobs.drupal.org is no longer available as of September 2026. There are no RSS items to parse. Marked the source stale and removed the earlier wildcard-feed claim.
- 2026-11-27: Live validation returned HTTP 200 but the RSS feed was malformed. The endpoint or feed parser needs review.

- 2026-11-26: Ported from historical behavior into the manifest adapter with synthetic fixtures.

