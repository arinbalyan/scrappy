# Crelate (ATS)

## Current integration
- Parser order:
  1. JSON API at `https://app.crelate.com/api3/jobs`
  2. Company seeds from env, config file, or search term
- Extracted fields: id, name, description, city, state_province, postal_code, country, is_remote, created_date

## Supported knobs
- `results_wanted`
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)

## Seed configuration
- **Env:** `SCRAPPY_CRELATE_SEEDS` — comma-separated company slugs (used as `X-Api-Key` header)
- **Config:** `config/company_slugs.yaml` under `crelate` key
- **Search fallback:** `--search` passed as slug if no env/config entry exists
- Seeds are capped at 8 (hardcoded limit, not using `SCRAPPY_ATS_MAX_SEEDS`)

## Constraints and breakpoints
- The company slug is used as the `X-Api-Key` header — each slug is an API key for the tenant.
- Max 8 seeds processed (hardcoded limit).
- 700ms delay between processing each seed.
- API filters for `published=true` only.
- Job URL is constructed as `app.crelate.com/portal/{slug}/job/{id}`.

## Debug/update playbook
1. Verify company slug (API key) is valid for the Crelate tenant.
2. Check the API at `app.crelate.com/api3/jobs?published=true` responds.
3. Validate location field names from the response.
4. Re-run `go test ./tests/scraper/crelate` and full suite.
