# Trakstar

## Current integration
- API/URL pattern: JSON API at `https://{slug}.hire.trakstar.com/api/v1/openings`
- Seed source: `SCRAPPY_TRAKSTAR_SEEDS` env var, or `config/company_slugs.yaml` entry, or `--search` term
- Parser order:
  1. JSON API response (bare array of `trakstarJob`)
- Extracted fields: title, company_name (slug), location (city/state/country), description, department, employment_type, remote flag, date_posted, compensation (salary_min/salary_max/currency), job_url

## Supported knobs
- `results_wanted`
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)
- Seed inputs: `--trakstar-seeds` / `SCRAPPY_TRAKSTAR_SEEDS`

## Constraints and breakpoints
- Slug must match the Trakstar tenant subdomain exactly (e.g. `acme` for `acme.hire.trakstar.com/api/v1/openings`)
- Response is a bare JSON array — not wrapped in an object
- Location can come from dedicated `city`/`state`/`country` fields or the freeform `location` field
- Salary defaults to yearly interval; currency defaults to USD if empty
- Remote detection uses a `remote` boolean pointer
- Description is HTML — stripped via `util.StripHTML`
- No pagination — all openings returned in one response

## Debug/update playbook
1. Verify the Trakstar API URL resolves (e.g. `https://{slug}.hire.trakstar.com/api/v1/openings`)
2. Check response is a bare JSON array and matches `trakstarJob` types
3. Validate salary fields and location fallback logic
4. Re-run `go test ./internal/scraper/ats-trakstar` and full suite
