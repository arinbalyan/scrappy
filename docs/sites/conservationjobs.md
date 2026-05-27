# ConservationJobs

## Current integration
- Parser order:
  1. RSS feed at `https://www.conservationjobboard.com/rss`
- Extracted fields: title, link, guid, description, pubDate

## Supported knobs
- `results_wanted`
- `search_term` — client-side filter on title + description (OR semantics)
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)

## Constraints and breakpoints
- Single RSS feed fetch — no pagination.
- Search term supports OR semantics (e.g., "conservation OR wildlife").
- No company name extraction — company info is embedded in the description.
- ID extracted from GUID or URL path segment.

## Debug/update playbook
1. Verify the RSS feed URL at `conservationjobboard.com/rss`.
2. Validate `<item>` structure (title, link, guid, description, pubDate).
3. Check search term parsing for OR-quoted strings.
4. Re-run `go test ./tests/scraper/conservationjobs` and full suite.
