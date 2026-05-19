# ZipRecruiter

## Current integration
- Source mode: listing HTML parsing.
- Current extraction: card id, job URL, title, company.

## Supported knobs
- `results_wanted`
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)

## Constraints and breakpoints
- Card markup and classes (`job_content`, `t_org_link`) can drift.
- Relative links or ad blocks can pollute extraction.
- Anti-bot pages may still return HTTP 200.

## Debug/update playbook
1. Confirm response body contains normal listing cards.
2. Validate URL extraction outputs absolute links; normalize when needed.
3. Add alternate card regex fallback if primary selector fails.
4. Re-run `go test ./tests/scraper/ziprecruiter` and full suite.
