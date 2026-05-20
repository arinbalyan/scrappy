# Wellfound

## Current integration
- Parser order:
  1. `application/ld+json` JobPosting extraction
  2. fallback listing HTML parsing
- Extracted fields: url, title, company, description, employment type, remote marker (default true), date_posted when available.

## Supported knobs
- `results_wanted`
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)

## Constraints and breakpoints
- Card markup and CSS classes are brittle.
- Relative vs absolute URLs can vary by page variant.
- Timestamp is currently scrape-time fallback (not source post time).

## Debug/update playbook
1. Capture raw response and validate card anchor/title/company pattern.
2. Normalize URL construction if relative links appear.
3. If source starts exposing structured data, prefer that path over regex parsing.
4. Re-run `go test ./tests/scraper/wellfound` and full suite.
