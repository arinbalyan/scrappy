# AcademicCareers

## Current integration
- Parser order:
  1. RSS feed at `https://www.academiccareers.com/rss`
- Extracted fields: title, link, guid, description, pubDate, dc:creator

## Supported knobs
- `results_wanted`
- `search_term` — client-side filter on title, description, and creator
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)

## Constraints and breakpoints
- RSS feed returns all current listings — no server-side search or pagination.
- Search term filtering is client-side (case-insensitive substring match on title + description + creator).
- PubDate parsing uses RFC 1123 / RFC 3339 / ISO 8601 fallback chain.
- Company name is extracted from `dc:creator` element.
- Description may contain HTML — stripped to plain text.
- Single-feed fetch, no CORS or auth requirements.

## Debug/update playbook
1. Confirm the RSS feed URL is still `academiccareers.com/rss`.
2. Validate `<item>` structure: title, link, guid, description, pubDate, dc:creator.
3. Check for any namespace changes (e.g., `dc:` prefix changes).
4. Re-run `go test ./tests/scraper/academiccareers` and full suite.
