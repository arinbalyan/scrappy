# FossJobs

## Current integration
- Parser order:
  1. RSS feed at `https://www.fossjobs.net/rss/all/`
- Extracted fields: title, link, guid, description, pubDate, category

## Supported knobs
- `results_wanted`
- `search_term` — client-side filter on title + description + category (OR semantics)
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)

## Constraints and breakpoints
- Single RSS feed fetch — no pagination.
- Search term supports OR semantics.
- No company name extraction.
- ID extracted from GUID or URL path segment.
- PubDate parsing handles timezone abbreviations (PDT, PST, EDT, EST, etc.) by replacing with numeric offsets.

## Debug/update playbook
1. Verify the RSS feed URL at `fossjobs.net/rss/all/`.
2. Validate `<item>` structure (title, link, guid, description, pubDate, category).
3. Check timezone abbreviation mapping for any new TZ codes.
4. Re-run `go test ./tests/scraper/fossjobs` and full suite.
