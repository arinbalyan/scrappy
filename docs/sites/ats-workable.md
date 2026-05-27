# Workable

## Current integration
- API/URL pattern: JSON API at `https://apply.workable.com/api/v1/widget/accounts/{slug}`
- Seed source: `SCRAPPY_WORKABLE_SEEDS` env var, or `config/company_slugs.yaml` entry, or `--search` term
- Parser order:
  1. JSON API response (`wbResponse` / `jobs` array)
- Extracted fields: title, company_name (slug), location (city/region/country via `locations` array or flat fields), department, employment_type (normalized: fulltime/parttime/contract/internship), remote flag (telecommuting), date_posted (published_on/created_at), job_url

## Supported knobs
- `results_wanted`
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)
- Seed inputs: `--workable-seeds` / `SCRAPPY_WORKABLE_SEEDS`

## Constraints and breakpoints
- Slug must match the Workable account subdomain (e.g. `acme` for `apply.workable.com/acme`)
- Employment type is normalized from various formats (`full-time` -> `fulltime`, `contractor` -> `contract`, etc.) — unrecognized values pass through as-is
- Location comes from `locations[0]` array (preferred) or flat `city`/`state`/`country` fields
- Job URL resolution priority: `url` > `shortlink` > constructed fallback
- No pagination — all jobs returned in one response
- Date_posted falls back to `created_at` if `published_on` is empty

## Debug/update playbook
1. Verify the Workable API URL resolves (e.g. `https://apply.workable.com/api/v1/widget/accounts/{slug}`)
2. Check response structure matches `wbResponse` types
3. Validate employment type normalization for edge cases
4. Re-run `go test ./internal/scraper/ats-workable` and full suite
