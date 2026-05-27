# AIJobs

## Current integration
- Parser order:
  1. HTML regex extraction from `https://aijobs.ai/remote` — finds `<a>` tags with href and title text
- Extracted fields: title, job URL, remote status (default true)

## Supported knobs
- `results_wanted`
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)

## Constraints and breakpoints
- Purely regex-based on listing HTML; no structured data or detail page fetching.
- Job IDs are sequential indices (aijobs-1, aijobs-2, ...), not stable across runs.
- No company name, description, location, or date extraction.

## Debug/update playbook
1. Examine `https://aijobs.ai/remote` for HTML anchor pattern changes.
2. Update `reJob` regex if link structure changes.
3. Re-run `go test ./tests/scraper/aijobs` and full suite.
