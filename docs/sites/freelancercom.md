# Freelancer.com

## Current integration
- Parser order:
  1. JSON API at `https://www.freelancer.com/api/projects/0.1/projects/active`
- Extracted fields: id, title, description, seo_url, type, currency, budget, time_submitted, location, owner

## Supported knobs
- `results_wanted` — capped at 50
- `search_term` — server-side via `query` param
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)

## Constraints and breakpoints
- Free public API — no authentication required.
- Single-page query with `limit` parameter (max 50).
- Results are projects (freelance work), not traditional employment.
- All jobs are tagged `IsRemote: true` by default.
- Compensation type is determined by project `type`: "hourly" → hourly interval, otherwise yearly.
- Owner `display_name` is mapped to `CompanyName`.
- Date is in Unix timestamp (`time_submitted`).

## Debug/update playbook
1. Verify the API at `freelancer.com/api/projects/0.1/projects/active`.
2. Check the `result` → `projects` array structure.
3. Validate project type → compensation interval mapping.
4. Re-run `go test ./tests/scraper/freelancercom` and full suite.
