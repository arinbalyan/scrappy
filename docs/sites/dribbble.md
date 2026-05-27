# Dribbble

## Current integration
- Parser order:
  1. RSS feed at `https://dribbble.com/jobs.rss`
- Extracted fields: title, link, guid, description, pubDate

## Supported knobs
- `results_wanted`
- `search_term` — client-side filter on title + description (OR semantics)
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)

## Constraints and breakpoints
- Single RSS feed fetch — no pagination.
- Uses streaming XML decoder (`xml.NewDecoder`).
- No company name extraction.
- Description is HTML — stripped to plain text.
- Search term supports OR semantics.

## Debug/update playbook
1. Verify the RSS feed URL at `dribbble.com/jobs.rss`.
2. Validate `<item>` structure (title, link, guid, description, pubDate).
3. Check search term parsing for OR-quoted strings.
4. Re-run `go test ./tests/scraper/dribbble` and full suite.
