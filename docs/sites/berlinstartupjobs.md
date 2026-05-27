# BerlinStartupJobs

## Current integration
- Parser order:
  1. RSS feed at `https://berlinstartupjobs.com/feed/`
- Extracted fields: title, link, guid, description, pubDate, category, dc:creator

## Supported knobs
- `results_wanted`
- `search_term` — client-side filter on title + description + category
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)

## Constraints and breakpoints
- Single RSS feed fetch — no pagination.
- Title format is `Job Title // Company Name` — company extracted after `//`.
- Location is hardcoded to "Berlin".
- Search term filtering is client-side (OR semantics for `search_term` split on " OR ").
- PubDate parsing uses RFC 822 / RFC 1123 / RFC 3339 fallback chain.
- ID is extracted from GUID or URL path segment.

## Debug/update playbook
1. Verify the RSS feed URL at `berlinstartupjobs.com/feed/`.
2. Validate `<item>` structure (title, link, guid, description, pubDate, category).
3. Check title format separator (`//`) for company extraction.
4. Re-run `go test ./tests/scraper/berlinstartupjobs` and full suite.
