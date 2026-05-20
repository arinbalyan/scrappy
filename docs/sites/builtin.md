# BuiltIn

## Current integration
- Parser order:
  1. `application/ld+json` JobPosting extraction
  2. fallback HTML card parsing
- Extracted fields: url, title, company, description, date_posted (when present in ld+json).

## Supported knobs
- `results_wanted`
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)

## Constraints and breakpoints
- City-specific BuiltIn pages can vary in HTML wrappers.
- Some pages include partial/empty structured data.
- Anti-bot pages can still return HTTP 200.

## Debug/update playbook
1. Check whether `application/ld+json` still emits `JobPosting` entries.
2. If structured data degrades, adjust fallback HTML selectors.
3. Validate URL normalization and date parsing after any selector changes.
4. Re-run `go test ./tests/scraper/builtin` and full suite.
