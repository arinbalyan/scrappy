# reed

## Current integration
- Parser order:
  1. HTML scraping: https://www.reed.co.uk/jobs
- Extracted fields: title, company, location, description, salary, job URL, date

## Supported knobs
- `results_wanted`
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)
- `search_term`, `location`
- 

## Constraints and breakpoints
- HTML scraping of reed.co.uk (UK job board). Extracts structured data embedded in search page HTML.

## Debug/update playbook
1. Fetch https://www.reed.co.uk/jobs with browser-like headers and examine HTML structure.
2. Update regex or selector patterns if site markup changes.
3. Verify anti-bot challenge detection is working if blocked.
4. Re-run `go test ./tests/scraper/reed` and full suite.
