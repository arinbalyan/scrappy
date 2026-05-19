# Google Jobs

## Current integration
- Source mode: **API-first equivalent via embedded structured data**.
- Parser order:
  1. `application/ld+json` blocks (`@type: JobPosting`)
  2. fallback to SERP HTML block parsing (`data-job-id` + class selectors)
- Query input:
  - prefers `google_search_term`
  - falls back to `search_term`

## Supported knobs
- `google_search_term`
- `search_term`
- `results_wanted`
- `retries`, `max_rps`, `site_rps[google]` (through shared transport + runtime controls)

## Constraints and breakpoints
- No stable public jobs API endpoint; parser depends on page payload shape.
- Consent/captcha/interstitial pages can return valid HTML but zero jobs.
- Class-name fallback can drift; ld+json is preferred when present.

## Debug/update playbook
1. Capture failing response body.
2. Check if any `<script type="application/ld+json">` still contains `JobPosting` objects.
3. If yes, update json projection fields first (prefer this over regex changes).
4. If no, update fallback selectors for `data-job-id` card extraction.
5. Re-run `go test ./tests/scraper/google` and full suite.
