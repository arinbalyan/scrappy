# Crunchboard

## Current integration
- Parser order:
  1. RSS feed at `https://www.crunchboard.com/jobs.rss`
- Extracted fields: title, link, guid, description, pubDate

## Supported knobs
- `results_wanted`
- `search_term` — client-side filter on title + description (OR semantics)
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)

## Constraints and breakpoints
- Single RSS feed fetch — no pagination.
- Uses streaming XML decoder (`xml.NewDecoder`) for memory efficiency.
- Search term supports OR semantics.
- No company name extraction.
- Description is HTML — stripped to plain text.
- PubDate parsing uses RFC 1123 / RFC 1123Z.

## Debug/update playbook
1. Verify the RSS feed URL at `crunchboard.com/jobs.rss`.
2. Validate `<item>` structure (title, link, guid, description, pubDate).
3. Check search term parsing for OR-quoted strings.
4. Re-run `go test ./tests/scraper/crunchboard` and full suite.
