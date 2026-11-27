# drupaljobs

Source id: drupaljobs
Family: feeds_api
Health: working
Endpoint: GET https://jobs.drupal.org/filtered-jobs/%25/%25/%25/%25/%25/feed
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
- The all-filters feed path uses percent-encoded wildcard segments.

## Break and fix log
- 2026-11-27: Live validation returned HTTP 200 but the RSS feed was malformed. The endpoint or feed parser needs review.
- 2026-11-27: Fixed by switching to the current all-filters feed path with encoded wildcard segments, and refreshed the synthetic fixture and golden records. Verified via tier 4 replay.

- 2026-11-26: Ported from historical behavior into the manifest adapter with synthetic fixtures.

