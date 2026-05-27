# Avature (ATS)

## Current integration
- Parser order:
  1. HTML scraping of `{slug}.avature.net/careers/SearchJobs/`
  2. Company seeds from env, config file, or search term
- Extracted fields: title, job URL (from link), location (from CSS class)

## Supported knobs
- `results_wanted`
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)

## Seed configuration
- **Env:** `SCRAPPY_AVATURE_SEEDS` — comma-separated company slugs
- **Config:** `config/company_slugs.yaml` under `avature` key
- **Search fallback:** `--search` passed as slug (must not look like a search phrase)
- Seeds are capped at `SCRAPPY_ATS_MAX_SEEDS` (default 20)

## Constraints and breakpoints
- If the search term looks like a free-text phrase (contains spaces or "OR"), the scraper returns early with an error — Avature needs a company slug like `colgate-palmolive`, not a search string.
- HTML-based parsing is fragile: relies on regex patterns for job links (`/careers/JobDetail/`), title attributes, and location CSS classes.
- Pagination: page-based with `jobOffset` and `jobRecordsPerPage=12`, up to 50 pages.
- Company name is derived from slug (`slug` → title-cased).
- Location extraction uses CSS class pattern matching (`"location"` or `"job-location"`) — fragile.

## Debug/update playbook
1. Verify company slug by visiting `{slug}.avature.net/careers/`.
2. Check that HTML structure still uses `/careers/JobDetail/` links and `data-js-job` attributes.
3. Update regex patterns if CSS classes change.
4. Re-run `go test ./tests/scraper/avature` and full suite.
