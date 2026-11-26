# jobylon

Source id: jobylon
Family: ats
Health: requires_slug
Endpoint: GET https://feed.jobylon.com/feeds/{slug}/?format=json
Parser: json array
Fetcher: plain
Rate: 30 requests per minute
Concurrency: 1
Robots: allow public company feeds; skip challenged or missing feeds
Attribution: Jobylon
Credentials: none
Slugs: example-co (seed placeholder)

## Behavior

- The feed URL is completed with a company seed slug.
- The payload is a top-level JSON array of jobs that can be read through common field aliases.
- Titles, descriptions, company names, URLs, and locations map directly from feed fields.
- The old scraper's URL synthesis from slug or numeric ID is outside the adapter; fixture URLs are explicit.
- Search-term filtering and per-seed deduplication remain outside the adapter.

## Quirks

- Feeds without a configured seed are skipped; a search phrase is never treated as a slug.
- Employment type, workspace type, dates, and skills stay outside the normalized record contract.

## Break and fix log

- 2026-11-26: Ported the public company feed with a slug manifest and synthetic array fixture. The source stays `requires_slug` because a company slug must be supplied before a request can be built.

## Live validation

- 2026-11-26: GET with the seed slug `example-co` returned HTTP 404 HTML; health stays `requires_slug` because a valid tenant seed is caller-supplied.
