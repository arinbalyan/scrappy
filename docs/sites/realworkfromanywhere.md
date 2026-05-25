# Real Work From Anywhere

## Current integration
- Endpoint: `https://www.realworkfromanywhere.com/rss.xml`
- Response shape: RSS feed with `<item>` blocks.
- Extracted fields: title, link, description, pubDate, category.
- ID extracted from last URL path segment, falls back to GUID or simple hash.
- `IsRemote` is always `true` — the board is remote-only.

## Supported knobs
- `search_term` — client-side filter on title + description
- `results_wanted`
- global resilience knobs: `retries`, `max_rps`, `site_rps`, proxy config

## Constraints and breakpoints
- Remote-only board — `IsRemote` is hardcoded to `true`.
- RSS feed contains all current listings — no server-side search or pagination.
- Search term filtering is client-side (case-insensitive substring match on title + description).
- PubDate parsing uses RFC 1123 / RFC 1123Z / YYYY-MM-DD fallback chain.
- ID fallback uses a simple hash of the URL when path segment and GUID are empty.
- No company name in the RSS feed; single-feed fetch.

## Debug/update playbook
1. Confirm the RSS feed URL is still `realworkfromanywhere.com/rss.xml`.
2. Validate `<item>` structure (title, link, description, pubDate, category).
3. Check ID extraction still works from URL path segment or GUID.
4. Re-run `go test ./tests/scraper/realworkfromanywhere` and then full suite.
