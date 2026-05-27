# Djinni

## Current integration
- Parser order:
  1. RSS feed at `https://djinni.co/jobs/rss/`
- Extracted fields: title, link, guid, description, pubDate, category

## Supported knobs
- `results_wanted`
- `search_term` — client-side filter on title + description + category
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)

## Constraints and breakpoints
- Single RSS feed fetch — no pagination.
- Djinni is a Ukrainian/European tech job board.
- Company name is not available in the RSS feed.
- Remote detection is heuristic: looks for "remote" in title or description.
- ID is extracted from GUID or URL last path segment, falling back to FNV-1a hash.
- PubDate parsing uses RFC 1123 / RFC 1123Z.
- Search terms are client-side (case-insensitive substring match).

## Debug/update playbook
1. Verify the RSS feed URL at `djinni.co/jobs/rss/`.
2. Validate `<item>` structure and tag extraction.
3. Check company name extraction options if the feed changes.
4. Re-run `go test ./tests/scraper/djinni` and full suite.
