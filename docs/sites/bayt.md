# Bayt

## Current integration
- Parser order:
  1. HTML scraping of `www.bayt.com/en/international/jobs/{search-slug}-jobs/`
- Extracted fields: title, company name, location, job URL

## Supported knobs
- `results_wanted`
- `search_term` — used to construct the search URL path slug
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)

## Constraints and breakpoints
- HTML-based parsing with `golang.org/x/net/html` — fragile if DOM structure changes.
- Parses `<li data-js-job>` elements for job cards.
- Company name from `<div class="t-nowrap p10l">` → `<span>`.
- Location from `<div class="t-mute t-small">`.
- Pagination: page-based (20 results per page), up to 10 pages.
- Search URL is built as `/en/international/jobs/{term}-jobs/` where term is search slug (spaces replaced by hyphens).
- ID is generated via `util.HashID("bayt-" + jobURL)`.
- May require residential proxy due to bot detection.

## Debug/update playbook
1. Fetch a search page and inspect the HTML structure for `data-js-job` elements.
2. Validate company name and location extraction from CSS classes.
3. Check pagination URL pattern (`?page=N`).
4. Re-run `go test ./tests/scraper/bayt` and full suite.
