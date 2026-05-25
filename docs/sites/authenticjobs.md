# AuthenticJobs

## Current integration
- Endpoint: `https://authenticjobs.com/api/`
- Authentication: `api_key` query param from `AUTHENTICJOBS_API_KEY` env var
- Response shape: JSON object with `listings` array.
- Pagination: page-based (`page` param), stops when an empty page is returned.

## Supported knobs
- `search_term` -> `keywords`
- `results_wanted`
- global resilience knobs: `retries`, `max_rps`, `site_rps`, proxy config

## Constraints and breakpoints
- **Requires `AUTHENTICJOBS_API_KEY` env var** — site is skipped with WARN if missing.
- No direct job URL in the API; constructed as `https://authenticjobs.com/jobs/{id}`.
- `telecommuting` field mapped to `IsRemote` (truthy values: `yes`, `true`).
- `post_date` format is `YYYY-MM-DD`.
- No company logo or salary data available in the API response.

## Debug/update playbook
1. Verify `AUTHENTICJOBS_API_KEY` is set and valid.
2. Confirm API response still uses `listings` as the root key.
3. Validate `telecommuting` field values (may have changed upstream).
4. Re-run `go test ./tests/scraper/authenticjobs` and then full suite.
