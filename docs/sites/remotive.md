# Remotive

## Current integration
- Endpoint: `https://remotive.com/api/remote-jobs`
- Optional query: `search`
- Extracted fields: id, title, company_name, candidate_required_location, publication_date, url, description, category.

## Supported knobs
- `search_term` -> `search`
- `results_wanted`
- global resilience knobs: `retries`, `max_rps`, `site_rps`, proxy config

## Constraints and breakpoints
- Root key (`jobs`) and field naming are external contracts.
- `publication_date` format can vary by API evolution.
- Category-to-jobtype mapping is lossy and may drift.

## Debug/update playbook
1. Validate root `jobs` array and required fields.
2. Validate `publication_date` parsing (RFC3339 first, fallback parser if drift appears).
3. Keep mapping tolerant to missing optional fields.
4. Re-run `go test ./tests/scraper/remotive` and then full suite.
