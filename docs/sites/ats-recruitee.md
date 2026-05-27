# Recruitee

## Current integration
- API/URL pattern: JSON API at `https://{slug}.recruitee.com/api/offers`
- Seed source: `SCRAPPY_RECRUITEE_SEEDS` env var, or `config/company_slugs.yaml` entry, or `--search` term
- Parser order:
  1. JSON API response (`recruiteeResponse` / `offers` array)
- Extracted fields: title, company_name (slug), location (city/state/country), description, department, remote flag, date_posted, compensation (salary_min/salary_max, currency), job_url (via careers_url + slug)

## Supported knobs
- `results_wanted`
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)
- Seed inputs: `--recruitee-seeds` / `SCRAPPY_RECRUITEE_SEEDS`

## Constraints and breakpoints
- Slug must match the Recruitee tenant subdomain exactly (e.g. `acme` for `acme.recruitee.com`)
- No pagination — all offers returned in one response
- Salary defaults to yearly interval; currency defaults to USD if empty
- Job URL construction depends on `careers_url` being present; falls back to `https://{slug}.recruitee.com/o/{slug}`
- Description is HTML — stripped via `util.StripHTML`

## Debug/update playbook
1. Verify the Recruitee API URL resolves (e.g. `https://{slug}.recruitee.com/api/offers`)
2. Check response structure matches `recruiteeResponse` types
3. Validate salary fields and currency parsing
4. Re-run `go test ./internal/scraper/ats-recruitee` and full suite
