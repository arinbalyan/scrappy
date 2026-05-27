# CareerBuilder

## Current integration
- Parser order:
  1. HTML scraping of `www.careerbuilder.com/jobs`
  2. Regex extraction of card-based data attributes
- Extracted fields: title, company, location, job URL, posted date

## Supported knobs
- `results_wanted`
- `search_term` — required, server-side via `keywords` param
- `location` — server-side via `location` param
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)

## Constraints and breakpoints
- **Search term is required** — empty search returns an error.
- HTML-based parsing — fragile if DOM structure or CSS class names change.
- Uses regex patterns for `data-results-title`, `data-company`, `data-location`, `data-posted-date`, `data-job-did`.
- Rate-limited to 2 requests/second via 500ms ticker.
- Pagination: page-based (`page_number`), up to 100 pages with up to 25 jobs/page.
- Date parsing handles relative formats ("Posted 2 days ago", "30+ days ago", "Posted Today", "Yesterday").
- Anti-bot challenge detection is enabled.

## Debug/update playbook
1. Fetch a search page and inspect the HTML `data-*` attribute structure.
2. Update regex patterns if data attributes or CSS classes change.
3. Verify relative date parsing for new phrase patterns.
4. Re-run `go test ./tests/scraper/careerbuilder` and full suite.
