# ats-comeet

## Current integration
- Parser order:
  1. ATS API: https://www.comeet.com/careers-api/2.0/company/{slug}/positions
- Extracted fields: title, company, location, description, site identifier

## Supported knobs
- `results_wanted`
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)
- Company slug seed via `SCRAPPY_COMEET_SEEDS` env var, `config/company_slugs.yaml`, or `--search` as fallback

## Seed resolution
- `ats.ResolveSeeds` with key `SCRAPPY_COMEET_SEEDS`
- Priority: env var > config file (yaml) > search term
- Max seeds capped by `SCRAPPY_ATS_MAX_SEEDS` (default 20)

## Constraints and breakpoints
- Requires company slug seed or config entry; empty results if no seeds available.
- Comeet career API with token query param. Returns job listings with location, department, company name. Description built from details array.

## Debug/update playbook
1. Verify API endpoint is reachable and returns expected JSON.
2. Check that `SCRAPPY_COMEET_SEEDS` or the config file has target company slugs.
3. Inspect job response structure for field name changes.
4. Re-run `go test ./tests/scraper/comeet` and full suite.
