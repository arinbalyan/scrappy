# jobspresso

## Current integration
- Parser order:
  1. RSS feed: https://jobspresso.co/feed/?post_type=job_listing
- Extracted fields: title, link, description, pubDate, category

## Supported knobs
- `results_wanted`
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)
- Search term filtering (client-side via title/description matching)

## Constraints and breakpoints
- RSS feed is single-page; no pagination support.
- RSS feed from jobspresso.co (remote tech jobs). Regex-based extraction of item fields.

## Debug/update playbook
1. Fetch https://jobspresso.co/feed/?post_type=job_listing and validate RSS item structure.
2. Adjust regex-based item extraction if XML structure changes.
3. Verify `pubDate` format if date parsing fails.
4. Re-run `go test ./tests/scraper/jobspresso` and full suite.
