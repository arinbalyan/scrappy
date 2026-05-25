# Golang Jobs

## Current integration
- Endpoint: `https://www.golangprojects.com/rss.xml`
- Response shape: RSS feed with `<item>` blocks.
- Extracted fields: title, link, description, pubDate, category.
- ID extracted from last URL path segment.

## Supported knobs
- `search_term` — client-side filter on title + description + category
- `results_wanted`
- global resilience knobs: `retries`, `max_rps`, `site_rps`, proxy config

## Constraints and breakpoints
- RSS feed contains all current listings — no server-side search or pagination.
- Search term filtering is client-side (case-insensitive substring match on title, description, and category).
- `golangprojects.com` may have feed availability issues; the site itself is a niche Go-specific board.
- PubDate parsing uses RFC 822 / RFC 1123 / RFC 3339 fallback chain.
- No company name or structured location in the RSS feed.

## Debug/update playbook
1. Confirm the RSS feed URL is still `golangprojects.com/rss.xml`.
2. Validate `<item>` structure (title, link, description, pubDate, category).
3. Check that `category` field still contains relevant tags.
4. Re-run `go test ./tests/scraper/golangjobs` and then full suite.
