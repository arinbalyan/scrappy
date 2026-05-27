# Comeet (ATS)

## Current integration
- Parser order:
  1. JSON API at `https://www.comeet.com/careers-api/2.0/company/{slug}/positions`
  2. Company seeds from env, config file, or search term
- Extracted fields: uid, id, name, company_name, url, url_active_page, location, details, department, time_updated

## Supported knobs
- `results_wanted`
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)

## Seed configuration
- **Env:** `SCRAPPY_COMEET_SEEDS` — comma-separated company slugs
- **Config:** `config/company_slugs.yaml` under `comeet` key
- **Search fallback:** `--search` passed as slug if no env/config entry exists
- Seeds are capped at `SCRAPPY_ATS_MAX_SEEDS` (default 20)

## Constraints and breakpoints
- API endpoint uses the `token` query parameter (currently empty — may need a real token).
- Location is extracted from the `Location.Name` field with brute-force remote detection.
- Description is assembled from the `Details` array (each `Value` joined by newline).
- Job URL prefers `URLActivePage` over `URL`.
- Company name comes from `company_name` field, falling back to slug.

## Debug/update playbook
1. Verify company slug works at `comeet.com/careers-api/2.0/company/{slug}/positions`.
2. Check if a token parameter is needed.
3. Validate location string and remote detection.
4. Re-run `go test ./tests/scraper/comeet` and full suite.
