# BambooHR (ATS)

## Current integration
- Parser order:
  1. JSON API at `{slug}.bamboohr.com/careers/list`
  2. Company seeds from env, config file, or search term
- Extracted fields: id, jobOpeningName, departmentLabel, location, employmentStatusLabel, minimumExperience, compensation, description

## Supported knobs
- `results_wanted`
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)

## Seed configuration
- **Env:** `SCRAPPY_BAMBOOHR_SEEDS` — comma-separated company slugs
- **Config:** `config/company_slugs.yaml` under `bamboohr` key
- **Search fallback:** `--search` passed as slug if no env/config entry exists
- Seeds are capped at `SCRAPPY_ATS_MAX_SEEDS` (default 20)

## Constraints and breakpoints
- API endpoint follows the pattern `{slug}.bamboohr.com/careers/list`.
- Location comes from structured `bamboohrLocation` object (city, state, country).
- Description is HTML — stripped to plain text.
- Job URL is constructed as `{slug}.bamboohr.com/careers/{id}`.
- Company name defaults to the slug.

## Debug/update playbook
1. Verify company slug by visiting `{slug}.bamboohr.com/careers/list`.
2. Check the `result` array structure in the JSON response.
3. Validate location field names and structure.
4. Re-run `go test ./tests/scraper/bamboohr` and full suite.
