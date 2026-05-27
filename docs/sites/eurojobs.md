# EuroJobs

## Current integration
- Parser order:
  1. RSS feed at `https://www.eurojobs.com/rss`
- Extracted fields: title, link, guid, description, pubDate

## Supported knobs
- `results_wanted`
- `search_term` — client-side filter on title + description (OR semantics)
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)

## Constraints and breakpoints
- Single RSS feed fetch — no pagination.
- Search term supports OR semantics.
- Company name is not available in the RSS feed.
- ID extracted from GUID or URL last path segment, falling back to FNV-1a hash.
- Description is HTML — stripped to plain text.
- PubDate parsing uses RFC 1123 / RFC 1123Z.

## Debug/update playbook
1. Verify the RSS feed URL at `eurojobs.com/rss`.
2. Validate `<item>` structure and tag extraction.
3. Check search term parsing for OR-quoted strings.
4. Re-run `go test ./tests/scraper/eurojobs` and full suite.
