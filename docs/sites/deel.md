# Deel (ATS)

## Current integration
- Parser order:
  1. JSON API at `https://api.letsdeel.com/rest/v2/ats/job-postings`
  2. Company seeds from env, config file, or search term
- Extracted fields: id, title, description, location, department, employment_type, created_at, url, apply_url, salary, remote, company_name

## Supported knobs
- `results_wanted`
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)

## Seed configuration
- **Env:** `SCRAPPY_DEEL_SEEDS` — comma-separated seeds (first seed is used as Bearer token)
- **Config:** `config/company_slugs.yaml` under `deel` key
- **Search fallback:** `--search` passed as slug (used as Bearer token)
- Seeds are capped at `SCRAPPY_ATS_MAX_SEEDS` (default 20)

## Constraints and breakpoints
- The first seed/slug is used as the Bearer token for API authentication.
- Only the first seed is processed (no multi-tenant iteration).
- Compensation interval mapped from `salary.interval` field.
- Description is HTML — stripped to plain text.
- Location comes from structured `deelLocation` object (city, state, country).

## Debug/update playbook
1. Verify the Deel API token is valid (set as first seed).
2. Check the API at `api.letsdeel.com/rest/v2/ats/job-postings` responds.
3. Validate compensation interval mapping for new interval strings.
4. Re-run `go test ./tests/scraper/deel` and full suite.
