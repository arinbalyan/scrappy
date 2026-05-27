# Fountain (ATS)

## Current integration
- Parser order:
  1. JSON API at `https://api.fountain.com/v2/openings`
  2. Company seeds from env, config file, or search term
- Extracted fields: id, title, description, location, location_string, department, team, type, employment_type, created_at, url, apply_url, is_remote, compensation

## Supported knobs
- `results_wanted`
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)

## Seed configuration
- **Env:** `SCRAPPY_FOUNTAIN_SEEDS` — comma-separated seeds (first seed used as Bearer token)
- **Config:** `config/company_slugs.yaml` under `fountain` key
- **Search fallback:** `--search` passed as slug (used as Bearer token)
- Seeds are capped at `SCRAPPY_ATS_MAX_SEEDS` (default 20)

## Constraints and breakpoints
- The first seed/slug is used as the Bearer token for API authentication.
- Only the first seed is processed (no multi-tenant iteration).
- Location prefers structured `fountainLocation` object with fallback to `location_string`.
- Compensation interval default is hourly (Fountain is gig/hourly-worker oriented).
- Employment type normalized from both `employment_type` and `type` fields.
- Description is HTML — stripped to plain text.

## Debug/update playbook
1. Verify the Fountain API token is valid (set as first seed).
2. Check the API at `api.fountain.com/v2/openings` responds.
3. Validate compensation interval mapping.
4. Re-run `go test ./tests/scraper/fountain` and full suite.
