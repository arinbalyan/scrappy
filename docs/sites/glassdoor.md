# Glassdoor

## Current integration
- Source mode: listing HTML parsing.
- Current extraction: `data-jobid`, title (`jobLink`), company (`EmployerProfile_compactEmployerName`).

## Supported knobs
- `results_wanted`
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)

## Constraints and breakpoints
- Class names and card structure are brittle and region-dependent.
- Country/domain matrix is partial today (hardening backlog item).
- Anti-bot pages can return 200 responses with non-job payloads.

## Debug/update playbook
1. Validate payload contains listing cards, not challenge/consent content.
2. Confirm `data-jobid` presence and title/company selectors.
3. If selectors drift, add alternate selector regex and keep existing one as fallback.
4. Re-run `go test ./tests/scraper/glassdoor` and then full suite.
