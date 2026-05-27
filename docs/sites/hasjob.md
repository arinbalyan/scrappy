# hasjob

## Current integration
- Parser order:
  1. RSS feed: https://hasjob.co/feed
- Extracted fields: title, content, location, link, published date

## Supported knobs
- `results_wanted`
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)
- Search term filtering (client-side via title/description matching)

## Constraints and breakpoints
- RSS feed is single-page; no pagination support.
- HasGeek job board RSS feed. Indian tech job board. Uses regex-based XML tag extraction.

## Debug/update playbook
1. Fetch https://hasjob.co/feed and validate RSS item structure.
2. Adjust regex-based item extraction if XML structure changes.
3. Verify `pubDate` format if date parsing fails.
4. Re-run `go test ./tests/scraper/hasjob` and full suite.
