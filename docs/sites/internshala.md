# internshala

## Current integration
- Parser order:
  1. HTML scraping: https://internshala.com/jobs
- Extracted fields: title, company, location, stipend/salary, duration, type

## Supported knobs
- `results_wanted`
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)
- `search_term`, `location`
- 

## Constraints and breakpoints
- HTML scraping of Internshala (Indian internship/job platform). Two URL modes: jobs and internships. Extracts from structured card HTML.

## Debug/update playbook
1. Fetch https://internshala.com/jobs with browser-like headers and examine HTML structure.
2. Update regex or selector patterns if site markup changes.
3. Verify anti-bot challenge detection is working if blocked.
4. Re-run `go test ./tests/scraper/internshala` and full suite.
