# wordpressjobs

## Current integration
- Parser order:
  1. RSS feed: https://{site}/feed/
- Extracted fields: title, link, description, pubDate

## Supported knobs
- `results_wanted`
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)
- Search term filtering (client-side via title/description matching)

## Constraints and breakpoints
- RSS feed is single-page; no pagination support.
- Scrapes WordPress job board RSS feeds. Configurable RSS URL for different WordPress sites running job board plugins.

## Debug/update playbook
1. Fetch https://{site}/feed/ and validate RSS item structure.
2. Adjust regex-based item extraction if XML structure changes.
3. Verify `pubDate` format if date parsing fails.
4. Re-run `go test ./tests/scraper/wordpressjobs` and full suite.
