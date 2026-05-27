# huggingfacejobs

## Current integration
- Parser order:
  1. HTML scraping: https://huggingface.co/jobs
- Extracted fields: title, job URL

## Supported knobs
- `results_wanted`
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)
- `search_term`, `location`
- 

## Constraints and breakpoints
- HTML regex extraction from huggingface.co/jobs listing page. Extracts anchor links. Minimal field extraction.

## Debug/update playbook
1. Fetch https://huggingface.co/jobs with browser-like headers and examine HTML structure.
2. Update regex or selector patterns if site markup changes.
3. Verify anti-bot challenge detection is working if blocked.
4. Re-run `go test ./tests/scraper/huggingfacejobs` and full suite.
