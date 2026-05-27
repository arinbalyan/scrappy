# gunio

## Current integration
- Parser order:
  1. HTML scraping: https://www.gun.io/jobs
- Extracted fields: title, job URL

## Supported knobs
- `results_wanted`
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)
- `search_term`, `location`
- 

## Constraints and breakpoints
- HTML regex extraction from gun.io listing page. Extracts anchor links with titles. No company name, description, or location extraction.

## Debug/update playbook
1. Fetch https://www.gun.io/jobs with browser-like headers and examine HTML structure.
2. Update regex or selector patterns if site markup changes.
3. Verify anti-bot challenge detection is working if blocked.
4. Re-run `go test ./tests/scraper/gunio` and full suite.
