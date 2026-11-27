# drupaljobs

Source id: drupaljobs
Family: feeds_api
Health: working
Endpoint: GET https://jobs.drupal.org/all-jobs/feed
Parser: rss
Fetcher: plain
Credentials: none
Rate: 20 requests per minute
Concurrency: 2
Robots: allow
Attribution: Drupal Jobs

## Quirks

- Feed is niche and may change or challenge automated clients.
- Company is derived from the job URL host when the feed has no company tag.

## Break and fix log
- 2026-11-27: Live validation returned HTTP 200 but the RSS feed was malformed. The endpoint or feed parser needs review.

- 2026-11-26: Ported from historical behavior into the manifest adapter with synthetic fixtures.

