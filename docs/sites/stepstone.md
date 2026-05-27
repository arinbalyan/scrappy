# stepstone

## Current integration
- Parser order:
  1. HTML scraping: https://www.stepstone.de
- Extracted fields: title, company, location, description, salary, date

## Supported knobs
- `results_wanted`
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)
- `search_term`, `location`
- 

## Constraints and breakpoints
- HTML scraping of StepStone (German job board). Domain configurable (default: www.stepstone.de). Extracts structured data from search results.

## Debug/update playbook
1. Fetch https://www.stepstone.de with browser-like headers and examine HTML structure.
2. Update regex or selector patterns if site markup changes.
3. Verify anti-bot challenge detection is working if blocked.
4. Re-run `go test ./tests/scraper/stepstone` and full suite.
