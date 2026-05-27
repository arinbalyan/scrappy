# BreezyHR (ATS)

## Current integration
- Parser order:
  1. JSON endpoint at `https://{slug}.breezy.hr/json`
  2. Company seeds from env, config file, or search term
- Extracted fields: id, friendly_id, name, title, description, url, location, department, category, published_date, creation_date

## Supported knobs
- `results_wanted`
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)

## Seed configuration
- **Env:** `SCRAPPY_BREEZYHR_SEEDS` — comma-separated company slugs
- **Config:** `config/company_slugs.yaml` under `breezyhr` key (alias: `breezy`)
- **Search fallback:** `--search` passed as slug if no env/config entry exists
- Seeds are capped at `SCRAPPY_ATS_MAX_SEEDS` (default 20)

## Constraints and breakpoints
- **Status: DEPRECATED — the `/json` endpoint now returns a 302 redirect followed by CloudFront 403 (WAF block).** The public job listing API has been removed. The scraper remains in the codebase but will return empty results.
- The scraper remains for reference until a new API endpoint or authentication method is identified.
- Title prefers `Name` field, falling back to `Title`.
- Department uses `Department` field with fallback to `Category.Name`.

## Debug/update playbook
1. Confirm the `/json` endpoint still returns 403/redirect.
2. If a new API endpoint is found, update `buildURL` and JSON types.
3. Re-run `go test ./tests/scraper/breezyhr` and full suite.
