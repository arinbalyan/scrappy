# wuzzuf

## Current integration
- Parser order:
  1. HTML scraping: https://wuzzuf.net/search/jobs/
- Extracted fields: title, company, location, description, date

## Supported knobs
- `results_wanted`
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)
- `search_term`, `location`
- 

## Constraints and breakpoints
- HTML scraping of Wuzzuf (Egypt/Middle East job board). URL-based search path construction.

## Debug/update playbook
1. Fetch https://wuzzuf.net/search/jobs/ with browser-like headers and examine HTML structure.
2. Update regex or selector patterns if site markup changes.
3. Verify anti-bot challenge detection is working if blocked.
4. Re-run `go test ./tests/scraper/wuzzuf` and full suite.
