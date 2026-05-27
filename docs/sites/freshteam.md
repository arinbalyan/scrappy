# Freshteam (ATS)

## Current integration
- Parser order:
  1. JSON API at `https://{slug}.freshteam.com/api/job_postings`
  2. Company seeds from env, config file, or search term
- Extracted fields: id, title, description, department, branch, type, remote, closing_date, created_at, applicant_apply_link

## Supported knobs
- `results_wanted`
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)

## Seed configuration
- **Env:** `SCRAPPY_FRESHTEAM_SEEDS` — comma-separated seeds (format: `slug:apiKey` or just `slug`)
- **Config:** `config/company_slugs.yaml` under `freshteam` key
- **Search fallback:** `--search` passed as slug (must not look like a search phrase)
- Seeds are capped at `SCRAPPY_ATS_MAX_SEEDS` (default 20)

## Constraints and breakpoints
- If the search term looks like a free-text phrase, the scraper returns early with an error.
- Seed format can include API key: `slug:apiKey` (colon-separated).
- API key sent as `Authorization: Bearer {apiKey}` header.
- Location comes from the `branch` field.
- Description is HTML — stripped to plain text.
- Job URL from `applicant_apply_link` with fallback to constructed URL.

## Debug/update playbook
1. Verify company slug and optional API key.
2. Check the API at `{slug}.freshteam.com/api/job_postings` responds.
3. Validate branch/location mapping.
4. Re-run `go test ./tests/scraper/freshteam` and full suite.
