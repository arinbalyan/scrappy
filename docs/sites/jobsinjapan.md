# jobsinjapan

## Current integration
- Parser order:
  1. RSS feed: https://jobsinjapan.com/feed/
- Extracted fields: title, link, guid, description, pubDate

## Supported knobs
- `results_wanted`
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)
- Search term filtering (client-side via title/description matching)

## Constraints and breakpoints
- RSS feed is single-page; no pagination support.
- RSS feed from jobsinjapan.com. Jobs in Japan focused on international/English-friendly positions.

## Debug/update playbook
1. Fetch https://jobsinjapan.com/feed/ and validate RSS item structure.
2. Adjust regex-based item extraction if XML structure changes.
3. Verify `pubDate` format if date parsing fails.
4. Re-run `go test ./tests/scraper/jobsinjapan` and full suite.
