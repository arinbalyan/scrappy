# Coroflot

## Current integration
- Parser order:
  1. RSS feed at `http://feeds.feedburner.com/Coroflot/AllJobs`
- Extracted fields: title, link, guid, description, pubDate

## Supported knobs
- `results_wanted`
- `search_term` — client-side filter on title + description (OR semantics)
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)

## Constraints and breakpoints
- Single RSS feed fetch via FeedBurner — no pagination.
- Title format is `Company is seeking a Job Title` — company extracted by regex.
- Location is extracted from description content via heuristic regex patterns ("location:", "city:", "based in").
- ID extracted from GUID or URL path segment.
- Search term supports OR semantics.
- Feed URL uses `http://` (non-HTTPS) via FeedBurner.

## Debug/update playbook
1. Verify the FeedBurner URL still serves the RSS feed.
2. Validate company name extraction regex for new title formats.
3. Check location extraction regex for new description patterns.
4. Re-run `go test ./tests/scraper/coroflot` and full suite.
