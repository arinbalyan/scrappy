# EcoJobs

## Current integration
- Endpoint: `https://www.ecojobs.com/rss.xml`
- Response shape: RSS feed with `<item>` blocks.
- Extracted fields: title, link, description, pubDate.
- ID extracted from URL path segment.

## Supported knobs
- `search_term` — client-side filter on title + description
- `results_wanted`
- global resilience knobs: `retries`, `max_rps`, `site_rps`, proxy config

## Constraints and breakpoints
- RSS feed contains all current listings — no server-side search or pagination.
- Search term filtering is client-side (case-insensitive substring match on title + description).
- PubDate parsing uses RFC 822 / RFC 1123 / RFC 3339 fallback chain.
- Single-feed fetch, no CORS or auth requirements.

## Debug/update playbook
1. Confirm the RSS feed URL is still `ecojobs.com/rss.xml`.
2. Validate `<item>` structure has not changed (title, link, description, pubDate).
3. Check CDATA handling for any new tag wrapping.
4. Re-run `go test ./tests/scraper/ecojobs` and then full suite.
