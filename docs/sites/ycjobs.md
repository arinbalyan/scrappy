# ycjobs

## Current integration
- Parser order:
  1. HTML scraping: https://www.ycombinator.com/jobs
- Extracted fields: title, company, URL

## Supported knobs
- `results_wanted`
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)
- `search_term`, `location`
- 

## Constraints and breakpoints
- HTML regex extraction from Y Combinator jobs page. Minimal extraction of title, company name, and apply URL.

## Debug/update playbook
1. Fetch https://www.ycombinator.com/jobs with browser-like headers and examine HTML structure.
2. Update regex or selector patterns if site markup changes.
3. Verify anti-bot challenge detection is working if blocked.
4. Re-run `go test ./tests/scraper/ycjobs` and full suite.
