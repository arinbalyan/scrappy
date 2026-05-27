# AndroidJobs

## Current integration
- Parser order:
  1. RSS feed at `https://androidjobs.io/jobs.rss`
- Extracted fields: title, link, guid, description, pubDate

## Supported knobs
- `results_wanted`
- `search_term` — client-side filter on title + description
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)

## Constraints and breakpoints
- Single RSS feed fetch — no pagination.
- Title format is parsed as `Job Title - Company - City` using ` - ` separator.
- Search term filtering is client-side (case-insensitive substring match).
- PubDate parsing uses RFC 1123 format.
- GUID is used as ID, falling back to URL path segment.

## Debug/update playbook
1. Verify the RSS feed URL at `androidjobs.io/jobs.rss`.
2. Validate `<item>` structure has not changed.
3. Check title format separator pattern (` - `).
4. Re-run `go test ./tests/scraper/androidjobs` and full suite.
