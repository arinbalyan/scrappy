# monster

## Current integration
- Parser order:
  1. HTML scraping: https://www.monster.com/jobs/search
- Extracted fields: title, company, location, job URL, date posted (relative)

## Supported knobs
- `results_wanted`
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)
- `search_term`, `location`
- 

## Constraints and breakpoints
- HTML scraping with regex extraction. Uses data-testid selectors. Paginates with results page number. Rate-limited. Date is relative ('2 days ago').

## Debug/update playbook
1. Fetch https://www.monster.com/jobs/search with browser-like headers and examine HTML structure.
2. Update regex or selector patterns if site markup changes.
3. Verify anti-bot challenge detection is working if blocked.
4. Re-run `go test ./tests/scraper/monster` and full suite.
